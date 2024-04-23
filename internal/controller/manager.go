package controller

import (
	"context"
	"fmt"
	"github.com/bogo-y/kubernetes-cronvpa-controller/api/v1beta1"
	"github.com/bogo-y/kubernetes-cronvpa-controller/internal/cronjob"
	"github.com/bogo-y/kubernetes-cronvpa-controller/internal/observe"
	"github.com/ringtail/go-cron"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	log "k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
	"time"
)

type Manager struct {
	Executor cronjob.Executor
	client   client.Client
	mv       sync.Mutex
	v2j      map[types.UID]map[string]interface{}
	mj       sync.Mutex
	j2v      map[string]*JobInfo
}

type JobInfo struct {
	vpaNN   types.NamespacedName
	vpaID   types.UID
	runOnce bool
}

func NewManager(client client.Client) *Manager {
	m := Manager{
		client: client,
		v2j:    make(map[types.UID]map[string]interface{}),
		j2v:    make(map[string]*JobInfo),
	}
	m.Executor = cronjob.NewExecutorImpl(nil, m.JobResultHandler)
	cronjob.SetClient(client)
	return &m
}

func (m *Manager) setJob(vid types.UID, cond *v1beta1.Condition, ref *v1beta1.TargetRef, namespace string, vpaName string) error {
	m.mv.Lock()
	defer m.mv.Unlock()
	_, ok := m.v2j[vid][cond.JobID]
	if ok {
		return nil
	}
	job, err := cronjob.JobFactory(cond, ref, namespace)
	if err != nil {
		return err
	}
	err = m.Executor.AddJob(job)
	if err != nil {
		return err
	}
	m.v2j[vid][cond.JobID] = struct{}{}
	cond.State = v1beta1.JobSubmitted
	m.mj.Lock()
	defer m.mj.Unlock()
	m.j2v[cond.JobID] = &JobInfo{
		vpaNN: types.NamespacedName{
			Namespace: namespace,
			Name:      vpaName,
		},
		vpaID:   vid,
		runOnce: cond.RunOnce,
	}
	return nil
}
func (m *Manager) autoRemove(vid types.UID, conds []v1beta1.Condition) {
	m.mv.Lock()
	defer m.mv.Unlock()
	for jid := range m.v2j[vid] {
		flag := false
		for i := range conds {
			if conds[i].JobID == jid {
				flag = true
				break
			}
		}
		if flag == false {
			m.Executor.RemoveByID(jid)
			delete(m.v2j[vid], jid)
		}
	}
}
func (m *Manager) JobSoftCheck(JobID string) bool {
	m.mj.Lock()
	defer m.mj.Unlock()
	m.mv.Lock()
	defer m.mv.Unlock()
	info, ok := m.j2v[JobID]
	if !ok {
		return false
	}
	_, ok = m.v2j[info.vpaID][JobID]
	if !ok {
		delete(m.j2v, JobID)
	}
	return ok
}
func (m *Manager) JobHardCheck(JobID string) (bool, v1beta1.JobState) {
	if !m.JobSoftCheck(JobID) {
		return false, ""
	}
	m.mj.Lock()
	defer m.mj.Unlock()
	info, ok := m.j2v[JobID]
	if !ok {
		return false, ""
	}
	instance := &v1beta1.Cronvpa{}
	err := m.client.Get(context.TODO(), info.vpaNN, instance)
	if err == nil {
		for i := range instance.Status.Conditions {
			if instance.Status.Conditions[i].JobID == JobID {
				return true, instance.Status.Conditions[i].State
			}
		}
	}
	m.mv.Lock()
	defer m.mv.Unlock()
	delete(m.v2j[info.vpaID], JobID)
	return false, ""
}
func (m *Manager) JobResultHandler(jr *cron.JobResult) {
	if m.JobSoftCheck(jr.JobId) == false {
		return
	}
	instance := &v1beta1.Cronvpa{}
	m.mj.Lock()
	defer m.mj.Unlock()
	info, ok := m.j2v[jr.JobId]
	if !ok {
		return
	}
	err := m.client.Get(context.TODO(), info.vpaNN, instance)
	if err != nil {
		return
	}
	for i := range instance.Status.Conditions {
		if instance.Status.Conditions[i].JobID == jr.JobId {
			e := jr.Error
			if e == nil {
				instance.Status.Conditions[i].State = v1beta1.JobSucceed
				instance.Status.Conditions[i].Message = fmt.Sprintf("job %s executed successfuly. %s", instance.Status.Conditions[i].JobName, jr.Msg)
			} else {
				instance.Status.Conditions[i].State = v1beta1.JobFailed
				instance.Status.Conditions[i].Message = fmt.Sprintf("job %s failed to executed. %s", instance.Status.Conditions[i].JobName, e)
			}
			instance.Status.Conditions[i].LastProbeTime = metav1.NewTime(time.Now())
			break
		}
	}
	_ = m.UpdateInstance(context.TODO(), instance)
}
func (m *Manager) UpdateInstance(ctx context.Context, i *v1beta1.Cronvpa) error {
	err := m.client.Update(ctx, i)
	if err != nil {
		log.Errorf("Failed to update status for cronvpa %s in namespace %s, because of %s.", i.Name, i.Namespace, err)
	}
	return err
}

func (m *Manager) GC() {
	observe.KubeExpiredJobsInCronEngineTotal.Set(0)
	observe.KubeSubmittedJobsInCronEngineTotal.Set(0)
	observe.KubeSuccessfulJobsInCronEngineTotal.Set(0)
	observe.KubeExpiredJobsInCronEngineTotal.Set(0)
	entry := m.Executor.ListJob()
	observe.KubeJobsInCronEngineTotal.Set(float64(len(entry)))
	current := make([]string, 0, len(entry))
	for _, e := range entry {
		current = append(current, e.Job.ID())
	}
	needRemove := make([]string, 0)
	for _, c := range current {
		ok, state := m.JobHardCheck(c)
		if !ok {
			needRemove = append(needRemove, c)
			observe.KubeExpiredJobsInCronEngineTotal.Add(1)
		}
		if state == v1beta1.JobSucceed {
			observe.KubeSuccessfulJobsInCronEngineTotal.Add(1)
		} else if state == v1beta1.JobFailed {
			observe.KubeFailedJobsInCronEngineTotal.Add(1)
		} else if state == v1beta1.JobSubmitted {
			observe.KubeSubmittedJobsInCronEngineTotal.Add(1)
		}
	}
	for i := range needRemove {
		m.Executor.RemoveByID(needRemove[i])
	}
}
func (m *Manager) Run(stopChan chan struct{}) {
	m.Executor.Run()
	ticker := time.NewTicker(time.Minute * 10)
	go func() {
		for {
			select {
			case <-ticker.C:
				log.Infof("Starting GC")
				m.GC()
			}
		}
	}()
	<-stopChan
	m.Executor.Stop()
}
