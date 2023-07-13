/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/upload"
	githubupload "github.com/takutakahashi/oci-image-operator/actor/github/pkg/upload"
)

// detectCmd represents the detect command
var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "A brief description of your command",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		workDir := os.Getenv("WORK_DIR")
		if workDir == "" {
			workDir = "/tmp/actor-base"
		}
		base, err := upload.Init(nil, upload.Opt{
			WatchPath:      workDir,
			ImageName:      os.Getenv("IMAGE_NAME"),
			ImageNamespace: os.Getenv("IMAGE_NAMESPACE"),
			ImageTarget:    os.Getenv("IMAGE_TARGET"),
		})
		if err != nil {
			logrus.Fatal(err)
		}
		input, err := base.GetInput(ctx)
		if err != nil {
			logrus.Fatal(err)
		}
		ghupload, err := githubupload.Init()
		if err != nil {
			logrus.Fatal(err)
		}
		out, err := ghupload.Output(ctx, input)
		if err != nil {
			logrus.Fatal(err)
		}
		image, err := base.GetImage(ctx)
		if err != nil {
			logrus.Fatal(err)
		}
		if err := base.UpdateImage(ctx, image, &out); err != nil {
			logrus.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(uploadCmd)
}
