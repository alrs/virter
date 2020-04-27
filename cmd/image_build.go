package cmd

import (
	log "github.com/sirupsen/logrus"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/LINBIT/virter/internal/virter"
	"github.com/LINBIT/virter/pkg/actualtime"
	"github.com/LINBIT/virter/pkg/isogenerator"
)

func imageBuildCommand() *cobra.Command {
	var vmID uint
	var provisionFile string
	var provisionValues []string

	buildCmd := &cobra.Command{
		Use:   "build base_image new_image",
		Short: "Build an image",
		Long: `Build an image by starting a VM, running a provisioning
step, and then committing the resulting volume.`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			baseImageName := args[0]
			newImageName := args[1]

			ctx, cancel := dockerContext()
			defer cancel()

			v, err := VirterConnect()
			if err != nil {
				log.Fatal(err)
			}
			defer v.Disconnect()

			publicKeys, err := loadPublicKeys()
			if err != nil {
				log.Fatal(err)
			}

			privateKeyPath := getPrivateKeyPath()
			privateKey, err := loadPrivateKey()
			if err != nil {
				log.Fatal(err)
			}

			shutdownTimeout := viper.GetDuration("time.shutdown_timeout")

			// DockerClient will be set later if needed
			tools := virter.ImageBuildTools{
				ISOGenerator:  isogenerator.ExternalISOGenerator{},
				PortWaiter:    newSSHPinger(),
				AfterNotifier: actualtime.ActualTime{},
			}

			vmConfig := virter.VMConfig{
				ImageName:     baseImageName,
				VMName:        newImageName,
				MemoryKiB:     1048576, // one gigabyte
				VCPUs:         1,
				VMID:          vmID,
				SSHPublicKeys: publicKeys,
			}

			dockerContainerConfig := virter.DockerContainerConfig{
				ContainerName: "virter-build-" + newImageName,
			}

			provOpt := virter.ProvisionOption{
				FilePath: provisionFile,
				Values:   provisionValues,
			}

			provisionConfig, err := virter.NewProvisionConfig(provOpt)
			if err != nil {
				log.Fatal(err)
			}
			if provisionConfig.NeedsDocker() {
				docker, err := dockerConnect()
				if err != nil {
					log.Fatal(err)
				}
				defer docker.Close()
				tools.DockerClient = docker
			}

			buildConfig := virter.ImageBuildConfig{
				DockerContainerConfig: dockerContainerConfig,
				SSHPrivateKeyPath:     privateKeyPath,
				SSHPrivateKey:         privateKey,
				ShutdownTimeout:       shutdownTimeout,
				ProvisionConfig:       provisionConfig,
			}

			err = v.ImageBuild(ctx, tools, vmConfig, buildConfig)
			if err != nil {
				log.Fatal(err)
			}
		},
	}

	buildCmd.Flags().StringVarP(&provisionFile, "provision", "p", "", "name of toml file containing provisioning steps")
	buildCmd.Flags().StringSliceVarP(&provisionValues, "set", "s", []string{}, "set/override provisioning steps")
	buildCmd.Flags().UintVarP(&vmID, "id", "", 0, "ID for VM which determines the IP address")
	buildCmd.MarkFlagRequired("id")

	return buildCmd
}