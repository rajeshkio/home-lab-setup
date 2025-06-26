package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/muhlba91/pulumi-proxmoxve/sdk/v7/go/proxmoxve"
	"github.com/muhlba91/pulumi-proxmoxve/sdk/v7/go/proxmoxve/vm"
	"github.com/pulumi/pulumi-command/sdk/go/command/remote"
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
	Gateway  string `yaml:"gateway"`
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

func createVMFromTemplate(ctx *pulumi.Context, provider *proxmoxve.Provider, name, vmName, nodeName, vmPassword, ipAddress, gateway string, vmTemplateId, memory, cpu int64) (*vm.VirtualMachine, error) {
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
			Dns: &vm.VirtualMachineInitializationDnsArgs{
				Domain: pulumi.String("local"),
				Servers: pulumi.StringArray{
					pulumi.String("192.168.90.1"),
					pulumi.String("8.8.8.8"),
				},
			},
			IpConfigs: &vm.VirtualMachineInitializationIpConfigArray{
				&vm.VirtualMachineInitializationIpConfigArgs{
					Ipv4: &vm.VirtualMachineInitializationIpConfigIpv4Args{
						Address: pulumi.String(ipAddress),
						Gateway: pulumi.String(gateway),
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
		return nil, err
	}
	return vmInstance, nil
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

func installK3SServer(ctx *pulumi.Context, hostIP, vmPassword string, vmDependency pulumi.Resource) (*remote.Command, error) {
	return remote.NewCommand(ctx, "server-prep", &remote.CommandArgs{
		Connection: &remote.ConnectionArgs{
			Host:     pulumi.String(hostIP),
			User:     pulumi.String("rajeshk"),
			Password: pulumi.String(vmPassword),
		},
		Create: pulumi.Sprintf(`
				sudo tee /etc/resolv.conf << 'EOF'
nameserver 192.168.90.1
EOF
				curl -sfL https://get.k3s.io | sh -s - server \
				--cluster-init --tls-san=%s --tls-san=$(hostname -I | awk '{print $1}') \
				--write-kubeconfig-mode 644 \
				--node-external-ip=%s
				sudo systemctl restart k3s
				sudo k3s kubectl wait --for=condition=Ready nodes --all --timeout=300s
				sudo cat /var/lib/rancher/k3s/server/node-token
				`, hostIP, hostIP),
	}, pulumi.DependsOn([]pulumi.Resource{vmDependency}))
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
		gateway := cfg.Require("gateway")

		var ubuntuTemplate UbuntuTemplate
		cfg.RequireObject("ubuntu-template", &ubuntuTemplate)

		var sleTemplate SLETemplate
		cfg.RequireObject("sle-template", &sleTemplate)

		ubuntuVM, err := createVMFromTemplate(ctx, provider, "ubuntu-test", ubuntuTemplate.VMName, proxmoxNode, vmPassword, ubuntuTemplate.IP+"/24", gateway, ubuntuTemplate.ID, ubuntuTemplate.Memory, ubuntuTemplate.CPU)
		if err != nil {
			return fmt.Errorf("cannot create %v VM: %w", ubuntuVM.Name, err)
		}

		var sleVMs []*vm.VirtualMachine
		var sleServerIPs []string

		for i := range sleTemplate.Count {
			serverNum := i + 1
			resourceName := fmt.Sprintf("sle-server-%d", serverNum)
			vmName := fmt.Sprintf("%s%d", sleTemplate.VMName, serverNum)
			serverIP := incrementIP(sleTemplate.IP, int(i))
			sleServerIPs = append(sleServerIPs, serverIP)
			ctx.Log.Info(fmt.Sprintf("Creating SLE K3s server %d with IP %s", serverNum, serverIP), nil)

			sleVM, err := createVMFromTemplate(ctx, provider, resourceName, vmName, proxmoxNode, vmPassword, serverIP+"/24", gateway, sleTemplate.ID, sleTemplate.Memory, sleTemplate.CPU)
			if err != nil {
				return fmt.Errorf("cannot create SLE VM %s: %w", vmName, err)
			}
			sleVMs = append(sleVMs, sleVM)
		}
		if len(sleVMs) > 0 {
			_, err := installK3SServer(ctx, sleServerIPs[0], vmPassword, sleVMs[0])
			if err != nil {
				return fmt.Errorf("cannot install K3s server: %w", err)
			}
		}

		ctx.Export("ubuntuVmId", ubuntuVM.ID())
		ctx.Export("number of k3s nodes", pulumi.Int(sleTemplate.Count))

		for i, sleVM := range sleVMs {
			ctx.Export(fmt.Sprintf("sleServer%dVmId", i+1), sleVM.VmId)
			ctx.Export(fmt.Sprintf("sleServer%dVmName", i+1), sleVM.Name)
			ctx.Export(fmt.Sprintf("k3sServer%dIP", i+1), pulumi.String(sleServerIPs[i]))
		}
		return nil
	})
}

// Helper function to format bytes into human-readable format
