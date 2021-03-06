package explorer

import (
	"fmt"
	"log"
	"os/exec"
	"reflect"
	"strconv"
	"time"

	logrusRotate "github.com/LazarenkoA/LogrusRotate"
	"github.com/prometheus/client_golang/prometheus"
)

type ExplorerAvailablePerformance struct {
	BaseRACExplorer
}

func (this *ExplorerAvailablePerformance) Construct(s Isettings, cerror chan error) *ExplorerAvailablePerformance {
	logrusRotate.StandardLogger().WithField("Name", this.GetName()).Debug("Создание объекта")

	this.summary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: this.GetName(),
			Help: "Доступная производительность хоста",
		},
		[]string{"host"},
	)

	this.settings = s
	this.cerror = cerror
	prometheus.MustRegister(this.summary)
	return this
}

func (this *ExplorerAvailablePerformance) StartExplore() {
	delay := reflect.ValueOf(this.settings.GetProperty(this.GetName(), "timerNotyfy", 10)).Int()
	logrusRotate.StandardLogger().WithField("delay", delay).WithField("Name", this.GetName()).Debug("Start")

	timerNotyfy := time.Second * time.Duration(delay)
	this.ticker = time.NewTicker(timerNotyfy)
	for {
		// Для обеспечения паузы. Логика такая, при каждой итерайии нам нужно лочить мьютекс, в конце разлочить, как только придет запрос на паузу этот же мьютекс будет залочен во вне
		// соответственно итерация будет на паузе ждать
		this.pause.Lock()
		func() {
			logrusRotate.StandardLogger().WithField("Name", this.GetName()).Trace("Старт итерации таймера")
			defer this.pause.Unlock()

			if licCount, err := this.getData(); err == nil {
				this.summary.Reset()
				for key, value := range licCount {
					this.summary.WithLabelValues(key).Observe(value)
				}
			} else {
				this.summary.Reset()
				this.summary.WithLabelValues("").Observe(0) // Для того что бы в ответе был AvailablePerformance, нужно дл атотестов
				log.Println("Произошла ошибка: ", err.Error())
			}

		}()
		<-this.ticker.C
	}
}

func (this *ExplorerAvailablePerformance) getData() (data map[string]float64, err error) {
	data = make(map[string]float64)

	// /opt/1C/v8.3/x86_64/rac process --cluster=ee5adb9a-14fa-11e9-7589-005056032522 list
	procData := []map[string]string{}

	param := []string{}
	param = append(param, "process")
	param = append(param, "list")
	param = append(param, fmt.Sprintf("--cluster=%v", this.GetClusterID()))

	cmdCommand := exec.Command(this.settings.RAC_Path(), param...)
	if result, err := this.run(cmdCommand); err != nil {
		logrusRotate.StandardLogger().WithError(err).Error()
		return data, err
	} else {
		this.formatMultiResult(result, &procData)
	}

	// У одного хоста может быть несколько рабочих процессов в таком случаи мы берем среднее арифметическое по процессам
	tmp := make(map[string][]int)
	for _, item := range procData {
		if perfomance, err := strconv.Atoi(item["available-perfomance"]); err == nil {
			tmp[item["host"]] = append(tmp[item["host"]], perfomance)
		}
	}
	for key, value := range tmp {
		for _, item := range value {
			data[key] += float64(item)
		}
		data[key] = data[key] / float64(len(value))
	}
	return data, nil
}

func (this *ExplorerAvailablePerformance) GetName() string {
	return "AvailablePerformance"
}
