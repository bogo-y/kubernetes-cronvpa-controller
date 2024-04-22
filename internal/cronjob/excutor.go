package cronjob

import (
	"github.com/ringtail/go-cron"
	log "k8s.io/klog/v2"
	"time"
)

const (
	maxOutOfDateTimeout = time.Minute * 5
)

type Executor interface {
	Run()
	Stop()
	AddJob(job CronJob) error
	Remove(job CronJob) error
	RemoveByID(id string)
	Update(job CronJob) error
	FindJob(jobID string) (bool, string)
	ListJob() []*cron.Entry
}

type ExecutorImpl struct {
	Engine *cron.Cron
}

func (ce *ExecutorImpl) AddJob(job CronJob) error {
	err := ce.Engine.AddJob(job.SchedulePlan(), job)
	if err != nil {
		log.Errorf("Failed to add job to engine,because of %v", err)
	}
	return err
}

func (ce *ExecutorImpl) ListJob() []*cron.Entry {
	entries := ce.Engine.Entries()
	return entries
}

func (ce *ExecutorImpl) FindJob(jobID string) (bool, string) {
	entries := ce.Engine.Entries()
	for _, e := range entries {
		if e.Job.ID() == jobID {
			// clean up out of date jobs when it reached maxOutOfDateTimeout
			if e.Next.Add(maxOutOfDateTimeout).After(time.Now()) {
				return true, ""
			}
			//log.Warningf("The job %s(job id %s) in cronhpa %s namespace %s is out of date.", jobID)
			return false, "JobTimeOut"
		}
	}
	return false, ""
}

func (ce *ExecutorImpl) Update(job CronJob) error {
	ce.Engine.RemoveJob(job.ID())
	err := ce.Engine.AddJob(job.SchedulePlan(), job)
	if err != nil {
		log.Errorf("Failed to update job to engine,because of %v", err)
	}
	return err
}

func (ce *ExecutorImpl) Remove(job CronJob) error {
	ce.Engine.RemoveJob(job.ID())
	return nil
}
func (ce *ExecutorImpl) RemoveByID(job string) {
	ce.Engine.RemoveJob(job)
}

func (ce *ExecutorImpl) Run() {
	ce.Engine.Start()
}

func (ce *ExecutorImpl) Stop() {
	ce.Engine.Stop()
}

func NewExecutorImpl(timezone *time.Location, handler func(job *cron.JobResult)) Executor {
	if timezone == nil {
		timezone = time.Now().Location()
	}
	c := &ExecutorImpl{
		Engine: cron.NewWithLocation(timezone),
	}
	c.Engine.AddResultHandler(handler)
	return c
}
