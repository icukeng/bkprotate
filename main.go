package main;

import (
	"log"
	"fmt"
	"bytes"
	"io/ioutil"
	"os"
	"gopkg.in/yaml.v2"
	"gopkg.in/gomail.v2"
	"crypto/tls"
	
	"path/filepath"

	"bkprotate/vector"

)


type Config struct {
	Mail* struct {
		Host string
		Port int
		Verify bool
		From string
		To []string
	}

	Dir string 
	Rules[] struct {
		P string
		I []int
	}
}

type Order struct { // report
	FileErr int // файлы с ошибками
	FileNone int // файлы, неучтенные ни в одном правиле
	Used int
	Unused int
	// TODO тут надо какую то статистику по примененым правилам
	// например
	//   новые файлы (нужно вести кеш)
	//   остуствие поступлений в таумаут
}

func main() {
	var config Config
	var order Order

	configFile, err := ioutil.ReadFile(os.Args[1])
	fatalist("Read config", err)
	err = yaml.Unmarshal(configFile, &config)
	fatalist("Unmarshall config", err)
	// read all maintained dir contents
	fileNames, err := filepath.Glob(config.Dir + "/*")
	fatalist("Directory read", err)

	vectors := make([]vector.Vector, len(config.Rules))
	for ix, rule := range config.Rules {
		vectors[ix] = vector.MakeVector(rule.I, rule.P, len(fileNames)/len(config.Rules))
	}
	// walking incoming files
	for _, fileName := range fileNames {
		fileStat := "none"
		for ix, _:= range vectors {
			// файл может не совпасть с данным шаблоном, 
			// потому что относится к другому шаблону
			if fileStat == "parsed" {
				stat := vectors[ix].MatchFile(fileName)
				if stat == "glob" {
					panic("Bad pattern") // TODO тут нужно в отчет ошибку передать
				} 
				if stat == "time" { fileStat = "err-time" }
				if stat == "ok" { fileStat = "err-double" }
			}
			if fileStat == "none" {
				stat := vectors[ix].AppendFile(fileName)
				if stat == "glob" {
					panic("Bad pattern") // TODO тут нужно в отчет ошибку передать
				} 
				if stat == "time" { fileStat = "err-time" }
				if stat == "ok" { fileStat = "parsed" }
			}
		} // vektor loop
		if fileStat == "none" {
			order.FileNone++
		}
		if fileStat == "err-double" || fileStat == "err-time" {
			order.FileErr++
		}
	}
	// основная работа
	for _, vtr := range vectors {
		// сейчас сортировка нужна до определения окон, т к первое окно берется от новейшего файла
		vtr.SortFiles()
		// инциализация массива окон
		vtr.FillWindows()
		// раскладка файлов по окнам
		vtr.ProcessFiles()
		// vtr.Files()
	}
	// отчет на почту
	// нужно отправить order, 
	// по кажому вектору количество используемых и неиспользуемых бекапов, 
	// список используемых файлов
	report := bytes.NewBufferString("")
	for _, vtr := range vectors {
		used := vtr.GetUsedFiles()
		unused := vtr.GetUnusedFiles()
		order.Used += len(used)
		order.Unused += len(unused)
		fmt.Fprintf(report, "Vector %s used: %d deleted: %d \n", vtr.Desc(), len(used), len(unused))
		for _, fn := range unused {
			os.Remove(fn)
		}
	}
	reporttitle := fmt.Sprintf("bkprotate used:%d deleted:%d orphan:%d error:%d", 
	order.Used, order.Unused, order.FileNone, order.FileErr)

	if config.Mail != nil {
		m := gomail.NewMessage();
		m.SetHeader("From", config.Mail.From)
		m.SetHeader("To", config.Mail.To...)
		m.SetHeader("Subject", reporttitle)
		m.SetBody("text/plain", report.String())

		d := gomail.Dialer{Host: config.Mail.Host, Port: config.Mail.Port, SSL: false}

		if config.Mail.Verify == false {
			d.TLSConfig = &tls.Config{InsecureSkipVerify:true}
		}

		err = d.DialAndSend(m)
		fatalist("gomail ",err)
	}

	fmt.Println(reporttitle)
	fmt.Println(report)
	fmt.Println("DONE")
}	
// --------------------------------------------------------
func fatalist(str string, err error) {
	if err != nil {
		log.Fatalf(str, err)
	}
}
