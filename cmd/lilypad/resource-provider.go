package lilypad

import (
	"fmt"

	"github.com/lilypad-tech/lilypad/pkg/executor/bacalhau"
	optionsfactory "github.com/lilypad-tech/lilypad/pkg/options"
	"github.com/lilypad-tech/lilypad/pkg/resourceprovider"
	"github.com/lilypad-tech/lilypad/pkg/system"
	"github.com/lilypad-tech/lilypad/pkg/web3"
	"github.com/spf13/cobra"
)

func newResourceProviderCmd() *cobra.Command {
	options := optionsfactory.NewResourceProviderOptions()

	resourceProviderCmd := &cobra.Command{
		Use:     "resource-provider",
		Short:   "Start the lilypad resource-provider service.",
		Long:    "Start the lilypad resource-provider service.",
		Example: "",
		RunE: func(cmd *cobra.Command, _ []string) error {

			network, _ := cmd.Flags().GetString("network")
			options, err := optionsfactory.ProcessResourceProviderOptions(options, network)
			if err != nil {
				return err
			}
			return runResourceProvider(cmd, options, network)
		},
	}

	optionsfactory.AddResourceProviderCliFlags(resourceProviderCmd, &options)

	return resourceProviderCmd
}

func runResourceProvider(cmd *cobra.Command, options resourceprovider.ResourceProviderOptions, network string) error {
	commandCtx := system.NewCommandContext(cmd)
	defer commandCtx.Cleanup()

	telemetry, err := configureTelemetry(commandCtx.Ctx, system.ResourceProviderService, network, options.Telemetry, options.Web3)
	if err != nil {
		fmt.Printf("failed to setup opentelemetry: %s", err)
	}
	commandCtx.Cm.RegisterCallbackWithContext(telemetry.Shutdown)
	tracer := telemetry.TracerProvider.Tracer(system.GetOTelServiceName(system.ResourceProviderService))

	web3SDK, err := web3.NewContractSDK(commandCtx.Ctx, options.Web3, tracer)
	if err != nil {
		return err
	}

	executor, err := bacalhau.NewBacalhauExecutor(options.Bacalhau)
	if err != nil {
		return err
	}

	// Ensure that our executor is available
	status, err := executor.IsAvailable()
	if !status || err != nil {
		return err
	}

	resourceProviderService, err := resourceprovider.NewResourceProvider(options, web3SDK, executor, tracer)
	if err != nil {
		return err
	}

	resourecProviderErrors := resourceProviderService.Start(commandCtx.Ctx, commandCtx.Cm)
	for {
		select {
		case err := <-resourecProviderErrors:
			commandCtx.Cleanup()
			return err
		case <-commandCtx.Ctx.Done():
			return nil
		}
	}
}
