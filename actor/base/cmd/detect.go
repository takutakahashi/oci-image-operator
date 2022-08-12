/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/takutakahashi/oci-image-operator/actor/base/pkg/detect"
)

// detectCmd represents the detect command
var detectCmd = &cobra.Command{
	Use:   "detect",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		logrus.Info("detect starting")
		workDir := os.Getenv("WORK_DIR")
		if workDir == "" {
			workDir = "/tmp/actor-base"
		}
		d, err := detect.Init(nil, detect.DetectOpt{
			WatchPath:      fmt.Sprintf("%s/output", workDir),
			ImageName:      os.Getenv("IMAGE_NAME"),
			ImageNamespace: os.Getenv("IMAGE_NAMESPACE"),
		})
		if err != nil {
			logrus.Fatal(err)
		}
		logrus.Fatal(d.RunHTTP(context.Background()))
	},
}

func init() {
	rootCmd.AddCommand(detectCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// detectCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// detectCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
