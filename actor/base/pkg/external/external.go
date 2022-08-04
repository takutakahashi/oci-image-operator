package external

import (
	"io"
	"os"

	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/base"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/check"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/upload"
)

func LoadCheckInput(r io.Reader) (check.CheckInput, error) {
	if r == nil {
		var err error
		r, err = os.Open(base.InWorkDir("input"))
		if err != nil {
			return check.CheckInput{}, err
		}

	}
	c := check.CheckInput{}
	err := base.MarshalJSON(&c, r)
	return c, err
}

func ExportCheckOutput(output check.CheckOutput, w io.Writer) error {
	if w == nil {
		var err error
		w, err = os.Create(base.InWorkDir("input"))
		if err != nil {
			return err
		}
	}
	return base.ParseJSON(&output, w)

}
func LoadUploadInput(r io.Reader) (upload.Input, error) {
	if r == nil {
		var err error
		r, err = os.Open(base.InWorkDir("input"))
		if err != nil {
			return upload.Input{}, err
		}

	}
	c := upload.Input{}
	err := base.MarshalJSON(&c, r)
	return c, err

}
func ExportUploadExport(output upload.Output, w io.Writer) error {
	if w == nil {
		var err error
		w, err = os.Create(base.InWorkDir("input"))
		if err != nil {
			return err
		}
	}
	return base.ParseJSON(&output, w)
}
