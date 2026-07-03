# Netsoft ZTNA - Packer OVA Build Template
# Build: packer build -var 'version=1.0.0' ubuntu-24.04.pkr.hcl

packer {
  required_plugins {
    vmware = {
      version = ">= 1.1.0"
      source  = "github.com/hashicorp/vmware"
    }
  }
}

# Variables
variable "version" {
  type    = string
  default = "1.0.0"
}

variable "iso_url" {
  type    = string
  default = "https://releases.ubuntu.com/24.04/ubuntu-24.04.2-live-server-amd64.iso"
}

variable "iso_checksum" {
  type    = string
  default = "sha256:bed7ca43b1e5e3c18b455e15ed8a6ea99761a044a229ab782c86cb0a0dd09d75"
}

variable "vm_name" {
  type    = string
  default = "netsoft-ztna"
}

variable "output_dir" {
  type    = string
  default = "output"
}

variable "wizard_binary" {
  type    = string
  default = "../netsoft-wizard"
}

variable "ssh_username" {
  type    = string
  default = "netsoft"
}

variable "ssh_password" {
  type    = string
  default = "netsoft"
}

# VMware ISO builder
source "vmware-iso" "netsoft-ztna" {
  # VM Specs
  vm_name              = "${var.vm_name}-${var.version}"
  display_name         = "Netsoft ZTNA v${var.version}"
  guest_os_type        = "ubuntu-64"
  disk_size            = 32768
  disk_adapter_type    = "pvscsi"
  disk_type_id         = "thin"
  memory               = 4096
  cpus                 = 2
  cores                = 2
  audio                = false
  usb                  = false

  # Network
  network_adapter_type = "vmxnet3"

  # ISO
  iso_url              = var.iso_url
  iso_checksum         = var.iso_checksum

  # HTTP directory for autoinstall
  http_directory       = "http"

  # Boot command for Ubuntu 24.04 autoinstall
  boot_wait            = "5s"
  boot_command = [
    "<wait5>c<wait>",
    "linux /casper/vmlinuz autoinstall ds=nocloud-net;s=http://{{ .HTTPIP }}:{{ .HTTPPort }}/ ",
    "ip=dhcp url=http://{{ .HTTPIP }}:{{ .HTTPPort }}/ubuntu-24.04.squashfs ",
    "--- quiet",
    "<enter>",
    "initrd /casper/initrd",
    "<enter>",
    "boot",
    "<enter>"
  ]

  # SSH connection for provisioning
  communicator          = "ssh"
  ssh_username          = var.ssh_username
  ssh_password          = var.ssh_password
  ssh_port              = 22
  ssh_timeout           = "30m"
  ssh_handshake_attempts = 100

  # Output
  format               = "ova"
  output_directory     = "${var.output_dir}/${var.vm_name}-${var.version}"
  vmdk_name            = "${var.vm_name}-disk"
  ova_name             = "${var.vm_name}-${var.version}"

  # Skip export if we want raw VMX instead of OVA
  skip_export          = false

  # Shutdown
  shutdown_command     = "echo '${var.ssh_password}' | sudo -S shutdown -P now"
  shutdown_timeout     = "10m"

  # VMware settings
  vmx_data = {
    "mks.enable3d"            = "FALSE"
    "svga.autodetect"         = "FALSE"
    "vhv.enable"              = "TRUE"
    "ethernet0.virtualDev"    = "vmxnet3"
    "scsi0.virtualDev"        = "pvscsi"
    "guestinfo.metadata"      = ""
    "guestinfo.userdata"      = ""
    "guestinfo.vendordata"    = ""
  }

  # Remove VMware tools warning
  vmx_remove_ethernet_interfaces = false
}

# Build
build {
  sources = ["source.vmware-iso.netsoft-ztna"]

  # ==========================================
  # Provisioning
  # ==========================================

  # 1. Wait for cloud-init to finish
  provisioner "shell" {
    inline = [
      "while [ ! -f /var/lib/cloud/instance/boot-finished ]; do sleep 2; done",
      "echo 'Cloud-init finished'"
    ]
  }

  # 2. System updates + base packages
  provisioner "shell" {
    environment_vars = [
      "DEBIAN_FRONTEND=noninteractive"
    ]
    inline = [
      "sudo apt-get update -qq",
      "sudo apt-get upgrade -y -qq",
      "sudo apt-get install -y -qq ca-certificates curl gnupg lsb-release ufw",
      "echo 'System updated'"
    ]
  }

  # 3. Install Docker CE
  provisioner "shell" {
    environment_vars = [
      "DEBIAN_FRONTEND=noninteractive"
    ]
    inline = [
      "sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg",
      "echo 'deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable' | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null",
      "sudo apt-get update -qq",
      "sudo apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-compose-plugin",
      "sudo systemctl enable docker",
      "sudo systemctl start docker",
      "echo 'Docker installed'"
    ]
  }

  # 4. Create netbird user
  provisioner "shell" {
    inline = [
      "sudo useradd -r -s /bin/false -G docker netbird || true",
      "sudo mkdir -p /opt/netsoft/{data,stack,web,scripts}",
      "sudo mkdir -p /etc/netsoft/certs",
      "sudo mkdir -p /var/log/netsoft",
      "sudo chown -R netbird:netbird /opt/netsoft /etc/netsoft/certs /var/log/netsoft",
      "echo 'Directories created'"
    ]
  }

  # 5. Upload wizard binary
  provisioner "file" {
    source      = var.wizard_binary
    destination = "/tmp/netsoft-wizard"
  }

  provisioner "shell" {
    inline = [
      "sudo mv /tmp/netsoft-wizard /usr/local/bin/netsoft-wizard",
      "sudo chmod 755 /usr/local/bin/netsoft-wizard",
      "echo 'Wizard binary installed'"
    ]
  }

  # 6. Upload web UI files
  provisioner "file" {
    source      = "../web"
    destination = "/tmp/web"
  }

  provisioner "shell" {
    inline = [
      "sudo cp -r /tmp/web/* /opt/netsoft/web/",
      "sudo rm -rf /tmp/web",
      "echo 'Web UI files installed'"
    ]
  }

  # 7. Upload scripts
  provisioner "file" {
    source      = "../scripts"
    destination = "/tmp/scripts"
  }

  provisioner "shell" {
    inline = [
      "sudo cp /tmp/scripts/first-boot.sh /opt/netsoft/scripts/first-boot.sh",
      "sudo cp /tmp/scripts/run-wizard.sh /opt/netsoft/scripts/run-wizard.sh",
      "sudo chmod 755 /opt/netsoft/scripts/*.sh",
      "sudo cp /tmp/scripts/netsoft-wizard.service /etc/systemd/system/netsoft-wizard.service",
      "sudo cp /tmp/scripts/netsoft-first-boot.service /etc/systemd/system/netsoft-first-boot.service",
      "sudo systemctl daemon-reload",
      "sudo systemctl enable netsoft-first-boot.service",
      "sudo rm -rf /tmp/scripts",
      "echo 'Scripts and services installed'"
    ]
  }

  # 8. SSH hardening
  provisioner "shell" {
    inline = [
      "sudo sed -i 's/^#PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config",
      "sudo sed -i 's/^#PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config",
      "sudo systemctl restart sshd",
      "echo 'SSH hardened'"
    ]
  }

  # 9. Firewall
  provisioner "shell" {
    inline = [
      "sudo ufw --force reset",
      "sudo ufw default deny incoming",
      "sudo ufw default allow outgoing",
      "sudo ufw allow 22/tcp comment 'SSH'",
      "sudo ufw allow 80/tcp comment 'HTTP redirect'",
      "sudo ufw allow 443/tcp comment 'HTTPS NetBird'",
      "sudo ufw allow 8080/tcp comment 'Wizard setup'",
      "sudo ufw allow 3478/tcp comment 'STUN'",
      "sudo ufw allow 3478/udp comment 'STUN'",
      "sudo ufw allow 10000/tcp comment 'Signal'",
      "sudo ufw --force enable",
      "echo 'Firewall configured'"
    ]
  }

  # 10. Cleanup
  provisioner "shell" {
    inline = [
      "sudo apt-get clean -qq",
      "sudo apt-get autoremove -y -qq",
      "sudo rm -rf /var/lib/apt/lists/*",
      "sudo rm -f /etc/machine-id",
      "sudo rm -f /var/lib/dbus/machine-id",
      "sudo truncate -s 0 /etc/hostname",
      "sudo dd if=/dev/zero of=/zero bs=1M || true",
      "sudo rm -f /zero",
      "echo 'Cleanup complete'"
    ]
  }

  # 11. Verify installation
  provisioner "shell" {
    inline = [
      "echo '=== Verification ==='",
      "ls -lh /usr/local/bin/netsoft-wizard",
      "docker --version",
      "docker compose version",
      "systemctl is-enabled netsoft-first-boot.service",
      "ls -la /opt/netsoft/",
      "ls /opt/netsoft/web/templates/",
      "echo '=== Verification Complete ==='"
    ]
  }
}
