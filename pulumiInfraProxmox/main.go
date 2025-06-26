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

func createVMFromTemplate(ctx *pulumi.Context, provider *proxmoxve.Provider, name, vmName, nodeName, vmPassword, ipAddress, gateway string, vmTemplateId, memory, cpu int64, useSSHKey bool) (*vm.VirtualMachine, error) {
	var userAccount *vm.VirtualMachineInitializationUserAccountArgs
	if useSSHKey {
		// For Ubuntu VM: Use SSH key from environment variable
		sshPublicKey := os.Getenv("SSH_PUBLIC_KEY")
		if sshPublicKey == "" {
			return nil, fmt.Errorf("SSH_PUBLIC_KEY environment variable is required for Ubuntu VM")
		}
		userAccount = &vm.VirtualMachineInitializationUserAccountArgs{
			Username: pulumi.String("rajeshk"),
			Keys: pulumi.StringArray{
				pulumi.String(sshPublicKey),
			},
		}
	} else {
		// For SLE VMs: Use password authentication
		userAccount = &vm.VirtualMachineInitializationUserAccountArgs{
			Username: pulumi.String("rajeshk"),
			Password: pulumi.String(vmPassword),
		}
	}

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
			UserAccount: userAccount,
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

func installHaProxy(ctx *pulumi.Context, lbIP string, vmDependency pulumi.Resource, k3sServerIPs []string) (*remote.Command, error) {

	var backendServers strings.Builder
	for i, serverIP := range k3sServerIPs {
		backendServers.WriteString(fmt.Sprintf("    server k3s-server-%d %s:6443 check\n", i+1, serverIP))
	}

	haProxyConfig := fmt.Sprintf(`
global
    daemon
    maxconn 4096
    log stdout local0

defaults
    mode tcp
    timeout connect 5000ms
    timeout client 50000ms
    timeout server 50000ms
    option tcplog
    log global

# K3s API Server Load Balancer
frontend k3s-api
    bind *:6443
    mode tcp
    default_backend k3s-servers

backend k3s-servers
    mode tcp
    balance roundrobin
%s
	`, backendServers.String())

	installCmd := fmt.Sprintf(`
		# Update package list
		sudo apt update
		
		# Install HAProxy
		sudo apt install -y haproxy
		
		# Backup original config
		sudo cp /etc/haproxy/haproxy.cfg /etc/haproxy/haproxy.cfg.backup
		
		# Create new HAProxy configuration
		sudo tee /etc/haproxy/haproxy.cfg << 'EOF'
%s
EOF
		
		# Enable and start HAProxy
		sudo systemctl enable haproxy
		sudo systemctl restart haproxy
		
		# Check HAProxy status
		sudo systemctl status haproxy --no-pager
		
		# Show listening ports
		sudo netstat -tulpn | grep haproxy
		
		echo "HAProxy installed and configured successfully"
		echo "K3s API accessible via: https://%s:6443"
	`, haProxyConfig, lbIP)

	resourceName := fmt.Sprintf("haproxy-install-%s", strings.ReplaceAll(lbIP, ".", "-"))

	cmd, err := remote.NewCommand(ctx, resourceName, &remote.CommandArgs{
		Connection: &remote.ConnectionArgs{
			Host:       pulumi.String(lbIP),
			User:       pulumi.String("rajeshk"),
			PrivateKey: pulumi.String(os.Getenv("PROXMOX_VE_SSH_PRIVATE_KEY")),
		},
		Create: pulumi.String(installCmd),
	}, pulumi.DependsOn([]pulumi.Resource{vmDependency}))
	return cmd, err
}

func installK3SServer(ctx *pulumi.Context, lbIP, vmPassword, serverIP string, vmDependency pulumi.Resource, isFirstServer bool, k3sToken pulumi.StringOutput, haproxyDependency pulumi.Resource) (*remote.Command, error) {
	var k3sCommand pulumi.StringInput

	if isFirstServer {
		k3sCommand = pulumi.Sprintf(`
			sudo tee /etc/resolv.conf << 'EOF'
nameserver 192.168.90.1
EOF
			curl -sfL https://get.k3s.io | sudo sh -s - server \
				--cluster-init --tls-san=%s --tls-san=$(hostname -I | awk '{print $1}') \
				--write-kubeconfig-mode 644
			sudo systemctl enable --now k3s
			sleep 100
			sudo k3s kubectl wait --for=condition=Ready nodes --all --timeout=300s
			sudo ls /var/lib/rancher/k3s/server/node-token
				`, lbIP)
	} else {
		k3sCommand = pulumi.Sprintf(`
			sudo tee /etc/resolv.conf << 'EOF'
nameserver 192.168.90.1
EOF
			# Wait for first server to be ready
			until curl -k -s https://%s:6443/ping; do
				echo "Waiting for first K3s server to be ready..."
				sleep 10
			done
			
			curl -sfL https://get.k3s.io | sudo sh -s - server \
			--server https://%s:6443 \
			--token %s \
			--tls-san=%s --tls-san=$(hostname -I | awk '{print $1}') \
			--write-kubeconfig-mode 644
			sudo systemctl enable --now k3s
			echo "K3s server joined cluster successfully"
		`, lbIP, lbIP, k3sToken, lbIP)
	}
	resourceName := fmt.Sprintf("k3s-server-%s", strings.ReplaceAll(serverIP, ".", "-"))
	cmd, err := remote.NewCommand(ctx, resourceName, &remote.CommandArgs{
		Connection: &remote.ConnectionArgs{
			Host:     pulumi.String(serverIP),
			User:     pulumi.String("rajeshk"),
			Password: pulumi.String(vmPassword),
		},
		Create: k3sCommand,
	}, pulumi.DependsOn([]pulumi.Resource{vmDependency}))
	return cmd, err
}

func getK3sToken(ctx *pulumi.Context, firstServerIP, vmPassword string, vmDependency pulumi.Resource) (*remote.Command, error) {
	resourceName := fmt.Sprintf("k3s-token-%s", strings.ReplaceAll(firstServerIP, ".", "-"))
	cmd, err := remote.NewCommand(ctx, resourceName, &remote.CommandArgs{
		Connection: &remote.ConnectionArgs{
			Host:     pulumi.String(firstServerIP),
			User:     pulumi.String("rajeshk"),
			Password: pulumi.String(vmPassword),
		},
		Create: pulumi.String(`
			# Wait for K3s to be fully ready and token file to exist
			#while [ ! -f /var/lib/rancher/k3s/server/node-token ]; do
			#	echo "Waiting for K3s token file..."
			#	sleep 5
			#done

			# Wait a bit more to ensure K3s is fully initialized
			#sleep 10

			sudo cat /var/lib/rancher/k3s/server/node-token
			`),
	}, pulumi.DependsOn([]pulumi.Resource{vmDependency}))
	return cmd, err
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

		ubuntuVM, err := createVMFromTemplate(ctx, provider, "ubuntu-test", ubuntuTemplate.VMName, proxmoxNode, vmPassword, ubuntuTemplate.IP+"/24", gateway, ubuntuTemplate.ID, ubuntuTemplate.Memory, ubuntuTemplate.CPU, true)
		if err != nil {
			return fmt.Errorf("cannot create %v VM: %w", ubuntuVM.Name, err)
		}

		var k3sServerIPs []string
		for i := range sleTemplate.Count {
			serverIP := incrementIP(sleTemplate.IP, int(i))
			k3sServerIPs = append(k3sServerIPs, serverIP)
		}

		haproxyCmd, err := installHaProxy(ctx, ubuntuTemplate.IP, ubuntuVM, k3sServerIPs)
		if err != nil {
			return fmt.Errorf("cannot install HAProxy: %w", err)
		}

		var sleVMs []*vm.VirtualMachine
		//var sleServerIPs []string
		var k3sCommands []*remote.Command
		var k3sServerToken pulumi.StringOutput

		for i := range sleTemplate.Count {
			serverNum := i + 1
			resourceName := fmt.Sprintf("sle-server-%d", serverNum)
			vmName := fmt.Sprintf("%s%d", sleTemplate.VMName, serverNum)
			serverIP := k3sServerIPs[i]
			ctx.Log.Info(fmt.Sprintf("Creating SLE K3s server %d with IP %s", serverNum, serverIP), nil)

			sleVM, err := createVMFromTemplate(ctx, provider, resourceName, vmName, proxmoxNode, vmPassword, serverIP+"/24", gateway, sleTemplate.ID, sleTemplate.Memory, sleTemplate.CPU, false)
			if err != nil {
				return fmt.Errorf("cannot create SLE VM %s: %w", vmName, err)
			}
			sleVMs = append(sleVMs, sleVM)

			if i == 0 {
				firstServerIP := serverIP
				k3sCmd, err := installK3SServer(ctx, ubuntuTemplate.IP, vmPassword, firstServerIP, sleVM, true, pulumi.String("").ToStringOutput(), haproxyCmd)
				if err != nil {
					return fmt.Errorf("cannot install K3s server on first node %s: %w", firstServerIP, err)
				}
				k3sCommands = append(k3sCommands, k3sCmd)
				tokenCmd, err := getK3sToken(ctx, firstServerIP, vmPassword, k3sCmd)
				if err != nil {
					return fmt.Errorf("cannot get K3s token from first server: %w", err)
				}
				k3sServerToken = tokenCmd.Stdout
			} else {
				k3sCmd, err := installK3SServer(ctx, ubuntuTemplate.IP, vmPassword, serverIP, sleVM, false, k3sServerToken, haproxyCmd)
				if err != nil {
					return fmt.Errorf("cannot install K3s on server %s: %w", serverIP, err)
				}
				k3sCommands = append(k3sCommands, k3sCmd)
			}
		}

		ctx.Export("ubuntuVmId", ubuntuVM.ID())
		ctx.Export("ubuntuVMName", ubuntuVM.Name)
		ctx.Export("number of k3s nodes", pulumi.Int(sleTemplate.Count))

		for i, sleVM := range sleVMs {
			ctx.Export(fmt.Sprintf("sleServer%dVmId", i+1), sleVM.VmId)
			ctx.Export(fmt.Sprintf("sleServer%dVmName", i+1), sleVM.Name)
			ctx.Export(fmt.Sprintf("k3sServer%dIP", i+1), pulumi.String(k3sServerIPs[i]))
		}

		for i, k3sCmd := range k3sCommands {
			ctx.Export(fmt.Sprintf("k3sServer%dInstallStatus", i+1), k3sCmd.Stdout)
		}
		return nil
	})
}

// Helper function to format bytes into human-readable format
