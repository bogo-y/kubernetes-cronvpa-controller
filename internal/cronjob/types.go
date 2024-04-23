package cronjob

import (
	"context"
	"errors"
	"fmt"
	"github.com/bogo-y/kubernetes-cronvpa-controller/api/v1beta1"
	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

var clt client.Client

func SetClient(c client.Client) {
	clt = c
}

type CronJob interface {
	ID() string
	Run() (msg string, error error)
	SchedulePlan() string
}

type TargetRef struct {
	NamespacedName types.NamespacedName
	GVK            schema.GroupVersionKind
}

type JobImpl struct {
	targetRef       TargetRef
	targetResources v1.ResourceRequirements
	id              string
	plan            string
	name            string
	containerName   string
}

func (cj *JobImpl) ID() string {
	return cj.id
}
func (cj *JobImpl) Name() string {
	return cj.name
}
func (cj *JobImpl) SchedulePlan() string {
	return cj.plan
}
func (cj *JobImpl) Equals(dj JobImpl) bool {
	if cj.id != dj.id || cj.plan != dj.plan || cj.name != dj.name || cj.targetRef != dj.targetRef {
		return false
	}
	return reflect.DeepEqual(cj.targetResources, dj.targetResources)
}
func (cj *JobImpl) Run() (string, error) {
	if cj.targetRef.GVK.Group == "apps" && cj.targetRef.GVK.Version == "v1" && cj.targetRef.GVK.Kind == "Deployment" {
		instance := &appsv1.Deployment{}
		err := clt.Get(context.TODO(), cj.targetRef.NamespacedName, instance)
		if err != nil {
			return "failed to get deployment", err
		}
		patch := client.MergeFrom(instance.DeepCopy())
		c := instance.Spec.Template.Spec.Containers
		found := false
		for i := range c {
			if c[i].Name == cj.containerName {
				c[i].Resources = cj.targetResources
				found = true
				break
			}
		}
		if !found {
			return "not found container", err
		}
		err = clt.Patch(context.TODO(), instance, patch)
		if err != nil {
			return "failed to patch deployment", err
		}
		return "success", nil
	} else if cj.targetRef.GVK.Kind == "Pod" {
		instance := &v1.Pod{}
		err := clt.Get(context.TODO(), cj.targetRef.NamespacedName, instance)
		if err != nil {
			return "failed to get pod", err
		}
		found := false
		for i := range instance.Spec.Containers {
			if instance.Spec.Containers[i].Name == cj.containerName {
				instance.Spec.Containers[i].Resources = cj.targetResources
				found = true
				break
			}
		}
		if !found {
			return "not found container", err
		}
		err = clt.Update(context.TODO(), instance)
		if err != nil {
			return "failed to update pod", err
		}
		return "success", nil
	} else {
		return "unsupported workload", errors.New(fmt.Sprintf("unable to scale %s", cj.targetRef.GVK.Kind))
	}
}

//	func CronJobFactory(vpajob *v1beta1.Job, ref *v1beta1.TargetRef, namespace string) (CronJob, error) {
//		splited := strings.Split(ref.ApiVersion, "/")
//		ret := JobImpl{
//			targetRef: TargetRef{
//				NamespacedName: types.NamespacedName{
//					Namespace: namespace,
//					Name:      ref.Name,
//				},
//				GVK: schema.GroupVersionKind{
//					Group:   splited[0],
//					Version: splited[1],
//					Kind:    ref.Kind,
//				},
//			},
//			targetResources: v1.ResourceRequirements{},
//			id:              uuid.New().String(),
//			plan:            vpajob.Schedule,
//			name:            vpajob.JobName,
//			containerName:   vpajob.TargetContainer,
//		}
//		err := checkRefValid(&ret.targetRef)
//		if err != nil {
//			return nil, err
//		}
//		return &ret, nil
//	}
func JobFactory(vpajob *v1beta1.Condition, ref *v1beta1.TargetRef, namespace string) (CronJob, error) {
	splited := strings.Split(ref.ApiVersion, "/")
	var g, v string
	if len(splited) == 1 {
		v = splited[0]
	} else {
		g = splited[0]
		v = splited[1]
	}
	ret := JobImpl{
		targetRef: TargetRef{
			NamespacedName: types.NamespacedName{
				Namespace: namespace,
				Name:      ref.Name,
			},
			GVK: schema.GroupVersionKind{
				Group:   g,
				Version: v,
				Kind:    ref.Kind,
			},
		},
		targetResources: v1.ResourceRequirements{},
		id:              uuid.New().String(),
		plan:            vpajob.Schedule,
		name:            vpajob.JobName,
		containerName:   vpajob.TargetContainer,
	}
	err := checkRefValid(&ret.targetRef)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}
func checkRefValid(r *TargetRef) error {
	if r.GVK.Version == "" || r.GVK.Group == "" || r.GVK.Kind == "" || r.NamespacedName.Namespace == "" || r.NamespacedName.Name == "" {
		return errors.New("any properties in ref could not be empty")
	}
	return nil
}
