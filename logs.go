package main

import (
	"fmt"
	"io"
	"os"
	"path"

	"mydocker/container"

	log "github.com/sirupsen/logrus"
)

func LogContainer(containerId string) {
	logFileLocation := path.Join(fmt.Sprintf(container.InfoLocFormat, containerId), container.GetLogFileName(containerId))
	logFile, err := os.Open(logFileLocation)
	defer logFile.Close()
	if err != nil {
		log.Errorf("Log container open file %s error %v", logFileLocation, err)
		return
	}
	content, err := io.ReadAll(logFile)
	if err != nil {
		log.Errorf("Log container read file %s error %v", logFileLocation, err)
		return
	}
	_, err = fmt.Fprint(os.Stdout, string(content))
	if err != nil {
		log.Errorf("Log container Fprint  error %v", err)
		return
	}
}
