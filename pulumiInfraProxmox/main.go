package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/muhlba91/pulumi-proxmoxve/sdk/v7/go/proxmoxve"
	"github.com/muhlba91/pulumi-proxmoxve/sdk/v7/go/proxmoxve/vm"

	//	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type UbuntuTemplate struct {
	Name     string `yaml:"name"`
	VMName   string `yaml:"vmName"`
	Password string `yaml:"password"`
	ID       int64  `yaml:"id"`
	DiskSize int64  `yaml:"disksize"`
	Memory   int64  `yaml:"memory"`
	CPU      int64  `yaml:"cpu"`
	IP       string `yaml:"ip"`
}

type SLETemplate struct {
	Name     string `yaml:"name"`
	Count    int64  `yaml:"count"`
	Password string `yaml:"password"`
	VMName   string `yaml:"vmName"`
	ID       int64  `yaml:"id"`
	DiskSize int64  `yaml:"disksize"`
	Memory   int64  `yaml:"memory"`
	CPU      int64  `yaml:"cpu"`
	IP       string `yaml:"ip"`
}

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
		Disks: &vm.VirtualMachineDiskArray{
			&vm.VirtualMachineDiskArgs{
				Interface: pulumi.String("scsi0"),
				//	DatastoreId: pulumi.String("nfs-iso"),
				Size:       pulumi.Int(32), // Match your template's disk size
				FileFormat: pulumi.String("raw"),
			},
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

func incrementIP(ip string, increment int) string {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return ip
	}
	lastoctet, err := strconv.Atoi(parts[3])
	if err != nil {
		return ip
	}
	newLastOctet := lastoctet + increment
	return fmt.Sprintf("%s.%s.%s.%d", parts[0], parts[1], parts[2], newLastOctet)
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
			return fmt.Errorf("error getting Proxmox nodes: %w", err)
		}

		//nodes, err := cluster.GetNodes(ctx, nil)
		// Iterate over the nodes and print their names.
		//if err != nil {
		//	return fmt.Errorf("error getting Proxmox nodes: %w", err)
		//}
		//ctx.Log.Info("Proxmox Nodes:", nil)
		//for i, nodeName := range nodes.Names {
		//	fmt.Printf("\n--- Node %d: %s ---\n", i, nodeName)

		//}
		cfg := config.New(ctx, "")

		proxmoxNode := cfg.Require("proxmox-node")
		vmPassword := cfg.Require("password")

		var ubuntuTemplate UbuntuTemplate
		cfg.RequireObject("ubuntu-template", &ubuntuTemplate)

		var sleTemplate SLETemplate
		cfg.RequireObject("sle-template", &sleTemplate)

		ubuntuVmId, ubuntuVmName, err := createVMFromTemplate(ctx, provider, "ubuntu-test", ubuntuTemplate.VMName, proxmoxNode, vmPassword, ubuntuTemplate.IP+"/24", ubuntuTemplate.ID, ubuntuTemplate.Memory, ubuntuTemplate.CPU)
		if err != nil {
			return fmt.Errorf("cannot create %v VM: %w", ubuntuVmId, err)
		}

		var sleVmIds []pulumi.IntOutput
		var sleVmNames []pulumi.StringOutput

		for i := range sleTemplate.Count {
			serverNum := i + 1
			resourceName := fmt.Sprintf("sle-server-%d", serverNum)
			vmName := fmt.Sprintf("%s%d", sleTemplate.VMName, serverNum)
			serverIP := incrementIP(sleTemplate.IP, int(i))
			fmt.Printf("Creating SLE server %d with IP %s\n", serverNum, serverIP)

			sleVmId, sleVmName, err := createVMFromTemplate(ctx, provider, resourceName, vmName, proxmoxNode, vmPassword, serverIP+"/24", sleTemplate.ID, sleTemplate.Memory, sleTemplate.CPU)
			if err != nil {
				return fmt.Errorf("cannot create SLE VM %s: %w", vmName, err)
			}
			sleVmIds = append(sleVmIds, sleVmId)
			sleVmNames = append(sleVmNames, sleVmName)
		}
		ctx.Export("ubuntuVmId", ubuntuVmId)
		ctx.Export("ubuntuVmName", ubuntuVmName)
		ctx.Export("number of k3s nodes", pulumi.Int(sleTemplate.Count))

		for i, vmId := range sleVmIds {
			ctx.Export(fmt.Sprintf("sleServer%dVmId", i+1), vmId)
			ctx.Export(fmt.Sprintf("sleServer%dVmName", i+1), sleVmNames[i])

		}
		return nil
	})
}

// Helper function to format bytes into human-readable format
