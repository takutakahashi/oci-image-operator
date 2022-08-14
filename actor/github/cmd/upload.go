/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/takutakahashi/oci-image-operator/actor/github/pkg/upload"
)

// detectCmd represents the detect command
var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "A brief description of your command",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		u, err := upload.Init()
		if err != nil {
			logrus.Fatal(err)
		}
		logrus.Error(u.Run(context.Background()))
	},
}

func init() {
	rootCmd.AddCommand(uploadCmd)
}
