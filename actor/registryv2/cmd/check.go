/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	basecheck "github.com/takutakahashi/oci-image-operator/actor/base/pkg/check"
	"github.com/takutakahashi/oci-image-operator/actor/registryv2/pkg/check"
	"github.com/takutakahashi/oci-image-operator/actor/registryv2/pkg/registryv2"
)

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		base, err := basecheck.Init(nil, basecheck.CheckOpt{
			ImageName:      os.Getenv("IMAGE_NAME"),
			ImageNamespace: os.Getenv("IMAGE_NAMESPACE"),
			ImageTarget:    os.Getenv("IMAGE_TARGET"),
		})
		if err != nil {
			logrus.Fatal(err)
		}
		r, err := registryv2.Init(nil, registryv2.Opt{
			Image: os.Getenv("REGISTRY_IMAGE_NAME"),
			Auth: &registryv2.Auth{
				Username: os.Getenv("REGISTRY_AUTH_USERNAME"),
				Password: os.Getenv("REGISTRY_AUTH_PASSWORD"),
			},
		})
		if err != nil {
			logrus.Fatal(err)
		}
		c, err := check.Init(r)
		if err != nil {
			logrus.Fatal(err)
		}

		// get existance
		input, err := base.GetInput(ctx)
		if err != nil {
			logrus.Fatal(err)
		}
		out, err := c.Output(input)
		if err != nil {
			logrus.Fatal(err)
		}

		// update image status
		image, err := base.GetImage(ctx)
		if err != nil {
			logrus.Fatal(err)
		}
		err = base.UpdateImage(ctx, image, out)
		if err != nil {
			logrus.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// checkCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// checkCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
