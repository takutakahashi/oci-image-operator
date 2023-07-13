/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"log"
	"os"

	"github.com/spf13/cobra"
	basedetect "github.com/takutakahashi/oci-image-operator/actor/base/pkg/detect"
	"github.com/takutakahashi/oci-image-operator/actor/github/pkg/detect"
)

// detectCmd represents the detect command
var detectCmd = &cobra.Command{
	Use:   "detect",
	Short: "A brief description of your command",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		base, err := basedetect.Init(nil, basedetect.DetectOpt{
			ImageName:      os.Getenv("IMAGE_NAME"),
			ImageNamespace: os.Getenv("IMAGE_NAMESPACE"),
		})
		if err != nil {
			log.Fatal(err)
		}
		d, err := detect.NewDetect(base)
		if err != nil {
			log.Fatal(err)
		}
		if err := d.Run(); err != nil {
			log.Fatal(err)
		}
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
