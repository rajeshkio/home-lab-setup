package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/muhlba91/pulumi-proxmoxve/sdk/v7/go/proxmoxve"
	"github.com/muhlba91/pulumi-proxmoxve/sdk/v7/go/proxmoxve/vm"
	"github.com/pulumi/pulumi-command/sdk/go/command/remote"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type VMTemplate struct {
	Name       string   `yaml:"name"`
	VMName     string   `yaml:"vmName"`
	ID         int64    `yaml:"id"`
	DiskSize   int64    `yaml:"disksize"`
	Memory     int64    `yaml:"memory"`
	CPU        int64    `yaml:"cpu"`
	IPConfig   string   `yaml:"ipconfig"`
	IPs        []string `yaml:"ips,omitempty"`
	Gateway    string   `yaml:"gateway"`
	Username   string   `yaml:"username"`
	Password   string   `yaml:"password,omitempty"`
	AuthMethod string   `yaml:"auth-method"`
	Count      int64    `yaml:"count,omitempty"`
}

func checkRequiredEnvVars() error {
	required := []string{
		"SSH_PUBLIC_KEY",
		"PROXMOX_VE_SSH_USERNAME",
		"PROXMOX_VE_ENDPOINT",
		"PROXMOX_VE_API_TOKEN",
		"PROXMOX_VE_SSH_PRIVATE_KEY",
	}

	var missingEnvVars []string
	for _, envVar := range required {
		if os.Getenv(envVar) == "" {
			missingEnvVars = append(missingEnvVars, envVar)
		}
	}
	if len(missingEnvVars) > 0 {
		return fmt.Errorf("missing required environment variables: %v", missingEnvVars)
	}
	return nil
}

func createVMFromTemplate(ctx *pulumi.Context, provider *proxmoxve.Provider, vmIndex int64, template VMTemplate, nodeName, gateway, password string) (*vm.VirtualMachine, error) {
	var userAccount *vm.VirtualMachineInitializationUserAccountArgs

	ctx.Log.Info(fmt.Sprintf("Creating VM with auth-method: %s, username: %s, password: %s", template.AuthMethod, template.Username, password), nil)
	if template.AuthMethod == "ssh-key" {
		userAccount = &vm.VirtualMachineInitializationUserAccountArgs{
			Username: pulumi.String(template.Username),
			Keys: pulumi.StringArray{
				pulumi.String(os.Getenv("SSH_PUBLIC_KEY")),
			},
		}
	} else {
		// For SLE VMs: Use password authentication
		userAccount = &vm.VirtualMachineInitializationUserAccountArgs{
			Username: pulumi.String(template.Username),
			Password: pulumi.String(password),
		}
	}

	var ipConfig *vm.VirtualMachineInitializationIpConfigArray
	if template.IPConfig == "static" {
		ctx.Export(fmt.Sprintf("vmIndex:%d", vmIndex), nil)
		ctx.Export(fmt.Sprintf("len of template.IPs:%d", len(template.IPs)), nil)
		if vmIndex >= int64(len(template.IPs)) {
			return nil, fmt.Errorf("not enough IPs provided for VM %d", vmIndex)
		}
		ipConfig = &vm.VirtualMachineInitializationIpConfigArray{
			&vm.VirtualMachineInitializationIpConfigArgs{
				Ipv4: vm.VirtualMachineInitializationIpConfigIpv4Args{
					Address: pulumi.String(template.IPs[vmIndex] + "/24"),
					Gateway: pulumi.String(gateway),
				},
			},
		}
	} else {
		ipConfig = nil
	}
	vmName := fmt.Sprintf("%s-%d", template.VMName, vmIndex)

	vmInstance, err := vm.NewVirtualMachine(ctx, template.Name+fmt.Sprintf("%d", vmIndex), &vm.VirtualMachineArgs{
		Name:     pulumi.String(vmName),
		NodeName: pulumi.String(nodeName),
		Memory: &vm.VirtualMachineMemoryArgs{
			Dedicated: pulumi.Int(template.Memory),
		},
		Cpu: &vm.VirtualMachineCpuArgs{
			Cores: pulumi.Int(template.CPU),
			Type:  pulumi.String("x86-64-v2-AES"),
		},
		Clone: &vm.VirtualMachineCloneArgs{
			NodeName: pulumi.String(nodeName),
			VmId:     pulumi.Int(template.ID),
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
			IpConfigs: ipConfig,
		},
		Started: pulumi.Bool(true),
	}, pulumi.Provider(provider))
	if err != nil {
		return nil, err
	}
	return vmInstance, nil
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

func setupProxmoxProvider(ctx *pulumi.Context) (*proxmoxve.Provider, error) {
	provider, err := proxmoxve.NewProvider(ctx, "proxmox-provider", &proxmoxve.ProviderArgs{
		Ssh: &proxmoxve.ProviderSshArgs{
			PrivateKey: pulumi.String(os.Getenv("PROXMOX_VE_SSH_PRIVATE_KEY")),
			Username:   pulumi.String(os.Getenv("PROXMOX_VE_SSH_USERNAME")),
		},
		Insecure: pulumi.Bool(true), // for self signed certificate
	})
	if err != nil {
		return nil, err
	}
	return provider, nil
}

func loadConfig(ctx *pulumi.Context) (string, string, string, []VMTemplate, error) {
	cfg := config.New(ctx, "")
	proxmoxNode := cfg.Require("proxmox-node")
	vmPassword := cfg.Require("password")
	gateway := cfg.Require("gateway")

	var templates []VMTemplate
	cfg.RequireObject("vm-templates", &templates)

	ctx.Export("vmPassword", pulumi.String(vmPassword))
	return proxmoxNode, vmPassword, gateway, templates, nil

}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {

		if err := checkRequiredEnvVars(); err != nil {
			return err
		}
		provider, err := setupProxmoxProvider(ctx)
		if err != nil {
			return fmt.Errorf("failed to setup Proxmox provider: %w", err)
		}
		fmt.Println(provider)

		proxmoxNode, vmPassword, gateway, templates, err := loadConfig(ctx)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		var allVMs []*vm.VirtualMachine
		for _, template := range templates {
			count := template.Count
			if count == 0 {
				count = 1
			}

			for i := range count {
				vm, err := createVMFromTemplate(ctx, provider, i, template, proxmoxNode, gateway, vmPassword)
				if err != nil {
					return fmt.Errorf("cannot create VM %s: %w", template.VMName, err)
				}
				allVMs = append(allVMs, vm)
				ctx.Log.Info(fmt.Sprintf("Created VM: %s", template.VMName), nil)
			}
		}

		fmt.Println(vmPassword)
		ctx.Export("totalVMsCreated", pulumi.Int(len(allVMs)))

		//
		// if err != nil {
		// 	return fmt.Errorf("cannot create %v VM: %w", template.Name, err)
		// }

		// var k3sServerIPs []string
		// for i := range sleTemplate.Count {
		// 	serverIP := incrementIP(sleTemplate.IP, int(i))
		// 	k3sServerIPs = append(k3sServerIPs, serverIP)
		// }

		// haproxyCmd, err := installHaProxy(ctx, ubuntuTemplate.IP, ubuntuVM, k3sServerIPs)
		// if err != nil {
		// 	return fmt.Errorf("cannot install HAProxy: %w", err)
		// }

		// var sleVMs []*vm.VirtualMachine

		// var k3sCommands []*remote.Command
		// var k3sServerToken pulumi.StringOutput

		// for i := range sleTemplate.Count {
		// 	serverNum := i + 1
		// 	resourceName := fmt.Sprintf("sle-server-%d", serverNum)
		// 	vmName := fmt.Sprintf("%s%d", sleTemplate.VMName, serverNum)
		// 	serverIP := k3sServerIPs[i]
		// 	ctx.Log.Info(fmt.Sprintf("Creating SLE K3s server %d with IP %s", serverNum, serverIP), nil)

		// 	sleVM, err := createVMFromTemplate(ctx, provider, resourceName, vmName, proxmoxNode, vmPassword, serverIP+"/24", gateway, sleTemplate.ID, sleTemplate.Memory, sleTemplate.CPU, false)
		// 	if err != nil {
		// 		return fmt.Errorf("cannot create SLE VM %s: %w", vmName, err)
		// 	}
		// 	sleVMs = append(sleVMs, sleVM)

		// 	if i == 0 {
		// 		firstServerIP := serverIP
		// 		k3sCmd, err := installK3SServer(ctx, ubuntuTemplate.IP, vmPassword, firstServerIP, sleVM, true, pulumi.String("").ToStringOutput(), haproxyCmd)
		// 		if err != nil {
		// 			return fmt.Errorf("cannot install K3s server on first node %s: %w", firstServerIP, err)
		// 		}
		// 		k3sCommands = append(k3sCommands, k3sCmd)
		// 		tokenCmd, err := getK3sToken(ctx, firstServerIP, vmPassword, k3sCmd)
		// 		if err != nil {
		// 			return fmt.Errorf("cannot get K3s token from first server: %w", err)
		// 		}
		// 		k3sServerToken = tokenCmd.Stdout
		// 	} else {
		// 		k3sCmd, err := installK3SServer(ctx, ubuntuTemplate.IP, vmPassword, serverIP, sleVM, false, k3sServerToken, haproxyCmd)
		// 		if err != nil {
		// 			return fmt.Errorf("cannot install K3s on server %s: %w", serverIP, err)
		// 		}
		// 		k3sCommands = append(k3sCommands, k3sCmd)
		// 	}
		// }

		// ctx.Export("ubuntuVmId", ubuntuVM.ID())
		// ctx.Export("ubuntuVMName", ubuntuVM.Name)
		// ctx.Export("number of k3s nodes", pulumi.Int(sleTemplate.Count))

		// for i, sleVM := range sleVMs {
		// 	ctx.Export(fmt.Sprintf("sleServer%dVmId", i+1), sleVM.VmId)
		// 	ctx.Export(fmt.Sprintf("sleServer%dVmName", i+1), sleVM.Name)
		// 	ctx.Export(fmt.Sprintf("k3sServer%dIP", i+1), pulumi.String(k3sServerIPs[i]))
		// }

		// for i, k3sCmd := range k3sCommands {
		// 	ctx.Export(fmt.Sprintf("k3sServer%dInstallStatus", i+1), k3sCmd.Stdout)
		// }
		return nil
	})
}

// Helper function to format bytes into human-readable format
