package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/base"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/check"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/detect"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/upload"
	"github.com/takutakahashi/oci-image-operator/api/v1beta1"
)

var dir string

func main() {
	op := os.Args[1]
	logrus.Info(op)
	dir = os.Getenv("WORK_DIR")
	if dir == "" {
		dir = "/tmp/actor-base"
	}
	switch op {
	case "detect":
		doDetect()
	case "check":
		doCheck()
	case "upload":
		doUpload()
	}
}

func doDetect() {
	for {
		seed := rand.Intn(60000)
		f := detect.DetectFile{
			Branches: map[string]string{
				"master": fmt.Sprintf("noopbranch%d", seed),
			},
			Tags: map[string]string{
				"latest/hash": fmt.Sprintf("nooptag%d", seed),
			},
		}
		w, err := os.Create(fmt.Sprintf("%s/output", dir))
		if err != nil {
			panic(err)
		}
		if err := base.ParseJSON(&f, w); err != nil {
			panic(err)
		}
		if _, err := http.Get("http://localhost:8080/"); err != nil {
			panic(err)
		}
		time.Sleep(1 * time.Minute)
	}
}

func doCheck() {
	for {
		r, err := os.Open(fmt.Sprintf("%s/input", dir))
		if err != nil {
			logrus.Error(err)
			continue
		}
		input := &check.CheckInput{}
		if err := base.MarshalJSON(input, r); err != nil {
			logrus.Error(err)
			continue
		}
		r.Close()
		for i := range input.Revisions {
			input.Revisions[i].Registry = "testreg"
			input.Revisions[i].Exist = v1beta1.ImageConditionStatusFalse
		}
		output := &check.CheckOutput{
			Revisions: input.Revisions,
		}
		w, err := os.Create(fmt.Sprintf("%s/output", dir))
		if err != nil {
			panic(err)
		}
		if err := base.ParseJSON(&output, w); err != nil {
			panic(err)
		} else {
			w.Close()
			os.Exit(0)
		}
		time.Sleep(10 * time.Second)
	}
}
func doUpload() {
	for {
		r, err := os.Open(fmt.Sprintf("%s/input", dir))
		if err != nil {
			logrus.Error(err)
			continue
		}
		input := &upload.Input{}
		if err := base.MarshalJSON(input, r); err != nil {
			logrus.Error(err)
			continue
		}
		r.Close()
		for i := range input.Builds {
			input.Builds[i].Succeeded = v1beta1.ImageConditionStatusTrue
		}
		output := &upload.Output{
			Builds: input.Builds,
		}
		w, err := os.Create(fmt.Sprintf("%s/output", dir))
		if err != nil {
			panic(err)
		}
		if err := base.ParseJSON(&output, w); err != nil {
			panic(err)
		} else {
			w.Close()
			os.Exit(0)
		}
		time.Sleep(10 * time.Second)
	}

}
