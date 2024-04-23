/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	autoscalingv1beta1 "github.com/bogo-y/kubernetes-cronvpa-controller/api/v1beta1"
	"github.com/bogo-y/kubernetes-cronvpa-controller/internal/observe"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	log "k8s.io/klog/v2"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
	"time"
)

// CronvpaReconciler reconciles a Cronvpa object
type CronvpaReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	manager *Manager
}

//+kubebuilder:rbac:groups=autoscaling.bogo.ac.cn.cronvpa.bogo.ac.cn,resources=cronvpas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=autoscaling.bogo.ac.cn.cronvpa.bogo.ac.cn,resources=cronvpas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=autoscaling.bogo.ac.cn.cronvpa.bogo.ac.cn,resources=cronvpas/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Cronvpa object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *CronvpaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.Info(fmt.Sprintf("Start to handle cronvpa %s in %s namespace", req.Name, req.Namespace))
	instance := &autoscalingv1beta1.Cronvpa{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			err = nil
		}
		return ctrl.Result{}, err
	}

	if instance.Status.TargetRef != instance.Spec.TargetRef {
		instance.Status.Conditions = nil
	}
	needUpdate := len(instance.Status.Conditions) != len(instance.Spec.Jobs)
	newConds := make([]autoscalingv1beta1.Condition, 0, len(instance.Spec.Jobs))
	for i, j := range instance.Spec.Jobs {
		for _, c := range instance.Status.Conditions {
			if c.JobName == j.JobName && c.Schedule == j.Schedule && c.RunOnce == j.RunOnce && c.TargetContainer == j.TargetContainer && reflect.DeepEqual(c.TargetResources, j.TargetResources) {
				newConds = append(newConds, c)
				break
			}
		}
		if len(newConds) < i {
			condition := autoscalingv1beta1.Condition{
				JobName:         j.JobName,
				JobID:           uuid.New().String(),
				Schedule:        j.Schedule,
				TargetResources: j.TargetResources,
				TargetContainer: j.TargetContainer,
				RunOnce:         j.RunOnce,
				State:           autoscalingv1beta1.JobInit,
				LastProbeTime:   metav1.Time{Time: time.Now()},
			}
			newConds = append(newConds, condition)
			needUpdate = true
		}
	}
	sort.Slice(newConds, func(i int, j int) bool {
		return newConds[i].JobName < newConds[j].JobName
	})
	instance.Status.Conditions = newConds
	if needUpdate {
		err = r.manager.UpdateInstance(ctx, instance)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	for i, _ := range instance.Status.Conditions {
		err = r.manager.setJob(instance.ObjectMeta.UID, &instance.Status.Conditions[i], &instance.Spec.TargetRef, instance.Namespace, instance.Name)
		if err != nil {
			log.Errorf("Failed to set job in status for cronvpa %s in namespace %s, because of %s.", instance.Name, instance.Namespace, err)
		}
	}
	r.manager.autoRemove(instance.UID, instance.Status.Conditions)
	err = r.manager.UpdateInstance(ctx, instance)
	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *CronvpaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&autoscalingv1beta1.Cronvpa{}).
		Complete(r)
	if err != nil {
		return err
	}
	r.manager = NewManager(r.Client)
	var stopChan chan struct{}
	go func(m *Manager, stopChan chan struct{}) {
		m.Run(stopChan)
		<-stopChan
	}(r.manager, stopChan)
	go func(cronManager *Manager, stopChan chan struct{}) {
		server := observe.NewWebServer(cronManager)
		server.Serve()
	}(r.manager, stopChan)
	return nil
}
