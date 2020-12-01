package play

import (
	"encoding/json"
	"fmt"
	"github.com/leochen2038/play/middleware/etcd"
	"github.com/robfig/cron/v3"
	"io/ioutil"
	"os"
	"time"
)

var (
	cronLastFileModTime int64
	cronJobs            = make(map[string]*cronJobWrap, 8)
	cronRunner          *cron.Cron
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
	cronRunner = cron.New()
	go func() {
		time.Sleep(1 * time.Second)
		cronRunner.Start()
	}()
}

func RegisterCronJob(name string, new func() CronJob) {
	cronJobs[name] = &cronJobWrap{name: name, newFunc: new}
}

func CronStop() {
	_ = <-cronRunner.Stop().Done()
}

func CronStartWithEtcd(etcd *etcd.EtcdAgent, key string, tryLocalFile string) {
	if data, err := etcd.GetEtcdValue(key); err != nil {
		if tryLocalFile != "" {
			if data, err = ioutil.ReadFile(tryLocalFile); err == nil {
				cronUpdate(data)
			}
		}
	} else {
		if err = cronUpdate(data); err == nil && tryLocalFile != "" {
			ioutil.WriteFile(tryLocalFile, data, 0644)
		}
	}

	etcd.StartWatchChange(key, func(data []byte) (err error) {
		if err = cronUpdate(data); err == nil && tryLocalFile != "" {
			err = ioutil.WriteFile(tryLocalFile, data, 0644)
		}
		return
	})
}

func CronStartWithFile(filename string, refashTickTime time.Duration) {
	if data, err := ioutil.ReadFile(filename); err == nil {
		cronUpdate(data)
	}

	if refashTickTime > 0 {
		if fileinfo, err := os.Stat(filename); err == nil {
			cronLastFileModTime = fileinfo.ModTime().Unix()
		}
		startCronWatchFileChange(filename, refashTickTime)
	}
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
		cronWatchFileChange(filename, refashTickTime)
	}()
}

func cronWatchFileChange(filename string, refashTickTime time.Duration) {
	var err error
	var fileinfo os.FileInfo
	var refashTicker = time.NewTicker(refashTickTime * time.Second)

	for {
		select {
		case <-refashTicker.C:
			if fileinfo, err = os.Stat(filename); err == nil && fileinfo.ModTime().Unix() > cronLastFileModTime {
				cronLastFileModTime = fileinfo.ModTime().Unix()
				if data, err := ioutil.ReadFile(filename); err == nil {
					cronUpdate(data)
				}
			}
		}
	}
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

func cronUpdate(configByte []byte) (err error) {
	var config map[string]string
	if err = json.Unmarshal(configByte, &config); err != nil {
		return
	}

	for _, job := range cronJobs {
		if job.runEntryId > 0 {
			if newSpec, ok := config[job.name]; !ok {
				cronRunner.Remove(job.runEntryId)
			} else if newSpec != job.spec {
				cronRemoveJob(job)
				cronAddJob(job, newSpec)
			}
			delete(config, job.name)
		}
	}

	for name, spec := range config {
		if job, ok := cronJobs[name]; ok {
			cronAddJob(job, spec)
		}
	}
	return
}
