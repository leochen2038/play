package play

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/robfig/cron/v3"
)

var (
	cronLastFileModTime int64
	cronJobs            = make(map[string]*cronJobWrap, 8)
	cronRunner          = cron.New()
)

type CronJob cron.Job

type CronConfig struct {
	Name, Spec string
}

type cronJobWrap struct {
	name       string
	spec       string
	runEntryId cron.EntryID
	newFunc    func() CronJob
}

func (j *cronJobWrap) Run() {
	defer func() { recover() }()
	j.newFunc().Run()
}

func init() {
	go func() {
		time.Sleep(3 * time.Second)
		cronRunner.Start()
	}()
}

func RegisterCronJob(name string, new func() CronJob) {
	cronJobs[name] = &cronJobWrap{name: name, newFunc: new}
}

func CronStop() {
	<-cronRunner.Stop().Done()
}

func CronStart() {
	cronRunner.Start()
}

func CronStartWithFile(filename string, refashTickTime time.Duration) (err error) {
	err = getConfigFromFile(filename)
	if refashTickTime > 0 {
		startCronWatchFileChange(filename, refashTickTime)
	}
	return
}

func startCronWatchFileChange(filename string, refashTickTime time.Duration) {
	go func() {
		defer func() {
			if panicInfo := recover(); panicInfo != nil {
				fmt.Println("start watch cron file painc:", panicInfo)
			}
			time.Sleep(5 * time.Second)
			startCronWatchFileChange(filename, refashTickTime)
		}()

		var refashTicker = time.NewTicker(refashTickTime * time.Second)
		for range refashTicker.C {
			if err := getConfigFromFile(filename); err != nil {
				fmt.Println("watch cron file error:", err)
			}
		}
	}()
}

func getConfigFromFile(filename string) (err error) {
	var fileinfo os.FileInfo
	if fileinfo, err = os.Stat(filename); err != nil {
		return
	}
	if fileinfo.ModTime().Unix() <= cronLastFileModTime {
		return
	}

	var data []byte
	if data, err = os.ReadFile(filename); err != nil {
		return
	}

	var config map[string]string
	if err = json.Unmarshal(data, &config); err != nil {
		return
	}

	cronLastFileModTime = fileinfo.ModTime().Unix()
	return _cronUpdate(config)
}

func cronRemoveJob(job *cronJobWrap) {
	if job.runEntryId > 0 {
		cronRunner.Remove(job.runEntryId)
		job.runEntryId = 0
		job.spec = ""
	}
}

func cronAddJob(job *cronJobWrap, newSpec string) {
	job.runEntryId, _ = cronRunner.AddJob(newSpec, job)
	job.spec = newSpec
}

func CronUpdate(config []CronConfig) (err error) {
	var c map[string]string = make(map[string]string, len(config))
	for _, job := range config {
		c[job.Name] = job.Spec
	}

	return _cronUpdate(c)
}

func _cronUpdate(config map[string]string) (err error) {
	for _, job := range cronJobs {
		if job.runEntryId > 0 {
			if newSpec, ok := config[job.name]; !ok {
				cronRemoveJob(job)
			} else if newSpec != job.spec {
				cronRemoveJob(job)
				cronAddJob(job, newSpec)
			}
			delete(config, job.name)
		}
	}
	var missJobs []string
	for name, spec := range config {
		if job, ok := cronJobs[name]; ok {
			cronAddJob(job, spec)
		} else {
			missJobs = append(missJobs, name)
		}
	}

	if len(missJobs) > 0 {
		err = fmt.Errorf("miss jobs: %v", missJobs)
	}
	return
}
