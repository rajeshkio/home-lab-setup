package main

import (
	"fmt"
	"log"
	"os"

	"github.com/muhlba91/pulumi-proxmoxve/sdk/v7/go/proxmoxve"
	"github.com/muhlba91/pulumi-proxmoxve/sdk/v7/go/proxmoxve/cluster"
	"github.com/muhlba91/pulumi-proxmoxve/sdk/v7/go/proxmoxve/vm"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func createVMFromTemplate(ctx *pulumi.Context, provider *proxmoxve.Provider, name, vmName, nodeName, vmPassword, ipAddress string, vmTemplateId, memory, cpu int64) (pulumi.IntOutput, pulumi.StringOutput, error) {
	vmInstance, err := vm.NewVirtualMachine(ctx, name, &vm.VirtualMachineArgs{
		Name:     pulumi.String(vmName),
		NodeName: pulumi.String(nodeName),
		Memory: &vm.VirtualMachineMemoryArgs{
			Dedicated: pulumi.Int(memory),
		},
		Cpu: &vm.VirtualMachineCpuArgs{
			Cores: pulumi.Int(cpu),
			Type:  pulumi.String("x86-64-v2-AES"),
		},
		Clone: &vm.VirtualMachineCloneArgs{
			NodeName: pulumi.String(nodeName),
			VmId:     pulumi.Int(vmTemplateId),
		},
		NetworkDevices: &vm.VirtualMachineNetworkDeviceArray{
			&vm.VirtualMachineNetworkDeviceArgs{
				Bridge:   pulumi.String("vmbr0"),
				Model:    pulumi.String("virtio"),
				Firewall: pulumi.Bool(true),
			},
		},
		Initialization: &vm.VirtualMachineInitializationArgs{
			DatastoreId: pulumi.String("nfs-iso"),
			UserAccount: &vm.VirtualMachineInitializationUserAccountArgs{
				Username: pulumi.String("rajeshk"),
				Password: pulumi.String(vmPassword),
			},
			IpConfigs: &vm.VirtualMachineInitializationIpConfigArray{
				&vm.VirtualMachineInitializationIpConfigArgs{
					Ipv4: &vm.VirtualMachineInitializationIpConfigIpv4Args{
						Address: pulumi.String(ipAddress),
						Gateway: pulumi.String("192.168.90.1"),
					},
				},
			},
		},
		//	BootOrders: pulumi.StringArray{
		//		pulumi.String("scsi0"),
		//		pulumi.String("net0"),
		//	},
		Started: pulumi.Bool(true),
	}, pulumi.Provider(provider))
	if err != nil {
		return pulumi.Int(0).ToIntOutput(), pulumi.String("").ToStringOutput(), err
	}
	return vmInstance.VmId, vmInstance.Name, nil

}
func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {

		provider, err := proxmoxve.NewProvider(ctx, "proxmox-provider", &proxmoxve.ProviderArgs{
			Ssh: &proxmoxve.ProviderSshArgs{
				PrivateKey: pulumi.String(os.Getenv("PROXMOX_VE_SSH_PRIVATE_KEY")),
				Username:   pulumi.String(os.Getenv("PROXMOX_VE_SSH_USERNAME")),
			},
		})
		if err != nil {
			return fmt.Errorf("error creating provider: %w", err)
		}

		nodes, err := cluster.GetNodes(ctx, nil)
		// Iterate over the nodes and print their names.
		if err != nil {
			return fmt.Errorf("error getting Proxmox nodes: %w", err)
		}
		ctx.Log.Info("Proxmox Nodes:", nil)
		for i, nodeName := range nodes.Names {
			fmt.Printf("\n--- Node %d: %s ---\n", i, nodeName)

		}
		ubuntuVmId, ubuntuVmName, err := createVMFromTemplate(ctx, provider, "ubuntu-test", "loadbalancer", "proxmox-2", "wifi123#", "192.168.90.200/24", 9000, 3048, 2)
		if err != nil {
			log.Fatal("Cannot create the VM Template vmName: %w", ubuntuVmName)
		}
		SleVmId, SlevmName, err := createVMFromTemplate(ctx, provider, "sle-test", "k3s-server", "proxmox-2", "wifi123#", "192.168.90.201/24", 9001, 3048, 2)
		if err != nil {
			log.Fatal("Cannot create the VM Template vmName: %w", SlevmName)
		}
		ctx.Export("ubuntuVmId", ubuntuVmId)
		ctx.Export("ubuntuVmName", ubuntuVmName)
		ctx.Export("SleVmId", SleVmId)
		ctx.Export("SlevmName", ubuntuVmName)
		return nil
	})
}

// Helper function to format bytes into human-readable format
