# Netsoft ZTNA - Packer Build Template (QEMU → OVA)
# Build: packer build -var 'version=1.0.0' ubuntu-24.04.pkr.hcl
# Output: .ova file (VMware compatible)

packer {
  required_plugins {
    qemu = {
      version = ">= 1.1.0"
      source  = "github.com/hashicorp/qemu"
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

# QEMU builder (works on any Linux, produces VMDK)
source "qemu" "netsoft-ztna" {
  vm_name          = "${var.vm_name}-${var.version}"
  disk_size        = 32768
  disk_interface   = "virtio"
  disk_cache       = "writeback"
  format           = "qcow2"
  net_device       = "virtio-net"
  memory           = 4096
  cpus             = 2
  qemu_binary      = "/usr/bin/qemu-system-x86_64"
  accelerator      = "kvm"
  use_default_display = false

  iso_url           = var.iso_url
  iso_checksum      = var.iso_checksum
  http_directory    = "http"

  boot_wait         = "10s"
  boot_key_interval = "10ms"
  boot_command = [
    "c<wait>",
    "linux /casper/vmlinuz autoinstall ds=nocloud-net;s=http://{{ .HTTPIP }}:{{ .HTTPPort }}/ ---<enter>",
    "initrd /casper/initrd<enter>",
    "boot<enter>"
  ]

  communicator          = "ssh"
  ssh_username          = var.ssh_username
  ssh_password          = var.ssh_password
  ssh_timeout           = "30m"
  ssh_handshake_attempts = 100

  shutdown_command     = "echo '${var.ssh_password}' | sudo -S shutdown -P now"
  shutdown_timeout     = "10m"

  output_directory     = "${var.output_dir}/${var.vm_name}-${var.version}"
  headless             = true
}

# Build
build {
  sources = ["source.qemu.netsoft-ztna"]

  # 1. Wait for cloud-init
  provisioner "shell" {
    inline = [
      "while [ ! -f /var/lib/cloud/instance/boot-finished ]; do sleep 2; done",
      "echo 'Cloud-init finished'"
    ]
  }

  # 2. System updates + base packages
  provisioner "shell" {
    environment_vars = ["DEBIAN_FRONTEND=noninteractive"]
    inline = [
      "sudo apt-get update -qq",
      "sudo apt-get upgrade -y -qq",
      "sudo apt-get install -y -qq ca-certificates curl gnupg lsb-release ufw",
      "echo 'System updated'"
    ]
  }

  # 3. Install Docker CE
  provisioner "shell" {
    environment_vars = ["DEBIAN_FRONTEND=noninteractive"]
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

  # 5. Wizard binary
  provisioner "file" {
    source      = var.wizard_binary
    destination = "/tmp/netsoft-wizard"
  }
  provisioner "shell" {
    inline = [
      "sudo mv /tmp/netsoft-wizard /usr/local/bin/netsoft-wizard",
      "sudo chmod 755 /usr/local/bin/netsoft-wizard"
    ]
  }

  # 6. Web UI files
  provisioner "file" {
    source      = "../web"
    destination = "/tmp/web"
  }
  provisioner "shell" {
    inline = [
      "sudo cp -r /tmp/web/* /opt/netsoft/web/",
      "sudo rm -rf /tmp/web"
    ]
  }

  # 7. Scripts + systemd
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
      "sudo rm -rf /tmp/scripts"
    ]
  }

  # 8. SSH hardening
  provisioner "shell" {
    inline = [
      "sudo sed -i 's/^#PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config",
      "sudo sed -i 's/^#PasswordAuthentication.*/PasswordAuthentication no/' /etc/ssh/sshd_config",
      "sudo systemctl restart sshd"
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
      "sudo ufw --force enable"
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
      "sync"
    ]
  }

  # 11. Verify
  provisioner "shell" {
    inline = [
      "echo '=== Verification ==='",
      "ls -lh /usr/local/bin/netsoft-wizard",
      "docker --version",
      "docker compose version",
      "systemctl is-enabled netsoft-first-boot.service",
      "ls /opt/netsoft/web/templates/",
      "echo '=== Verification Complete ==='"
    ]
  }

  # 12. Post-build: convert QCOW2 → VMDK → OVA
  provisioner "shell-local" {
    environment_vars = [
      "OUTDIR=${var.output_dir}/${var.vm_name}-${var.version}",
      "VMNAME=${var.vm_name}-${var.version}",
      "DISK_GB=32"
    ]
    inline = [
      "echo '=== Creating OVA from QCOW2 ==='",
      "cd \"$OUTDIR\"",
      "QCOW2=$(ls *.qcow2 | head -1)",
      "echo \"Source: $QCOW2\"",

      "# Convert to VMDK (streamOptimized for VMware)",
      "qemu-img convert -O vmdk -o subformat=streamOptimized \"$QCOW2\" \"$${VMNAME}-disk.vmdk\"",
      "echo \"VMDK: $(ls -lh $${VMNAME}-disk.vmdk | awk '{print $5}')\"",

      "# Build OVF descriptor",
      "cat > \"$${VMNAME}.ovf\" << 'OVFEOF'",
      "<?xml version=\"1.0\" encoding=\"UTF-8\"?>",
      "<Envelope vmw:buildId=\"build-1\" xmlns=\"http://schemas.dmtf.org/ovf/envelope/1\" xmlns:vmw=\"http://www.vmware.com/schema/ovf\" xmlns:vssd=\"http://schemas.dmtf.org/wbem/wscim/1/cim-schema/2/SystemVirtualizationSettings\" xmlns:rasd=\"http://schemas.dmtf.org/wbem/wscim/1/cim-schema/2/ResourceAllocationSettingData\">",
      "  <References>",
      "    <File ovf:href=\"$${VMNAME}-disk.vmdk\" ovf:id=\"file1\" ovf:size=\"0\"/>",
      "  </References>",
      "  <DiskSection>",
      "    <Info>Virtual disk information</Info>",
      "    <Disk ovf:capacity=\"$${DISK_GB}\" ovf:capacityAllocationUnits=\"byte * 2^30\" ovf:diskId=\"vmdisk1\" ovf:fileRef=\"file1\" ovf:format=\"http://www.vmware.com/interfaces/specifications/vmdk.html#streamOptimized\"/>",
      "  </DiskSection>",
      "  <NetworkSection>",
      "    <Info>Logical networks</Info>",
      "    <Network ovf:name=\"VM Network\">",
      "      <Description>The VM Network</Description>",
      "    </Network>",
      "  </NetworkSection>",
      "  <VirtualSystem ovf:id=\"$${VMNAME}\">",
      "    <Info>Netsoft ZTNA Virtual Appliance</Info>",
      "    <Name>$${VMNAME}</Name>",
      "    <OperatingSystemSection ovf:id=\"100\">",
      "      <Info>Ubuntu 24.04 LTS</Info>",
      "    </OperatingSystemSection>",
      "    <VirtualHardwareSection>",
      "      <Info>Virtual hardware requirements</Info>",
      "      <System>",
      "        <vssd:ElementName>Virtual Hardware Family</vssd:ElementName>",
      "        <vssd:InstanceID>0</vssd:InstanceID>",
      "        <vssd:VirtualSystemIdentifier>$${VMNAME}</vssd:VirtualSystemIdentifier>",
      "        <vssd:VirtualSystemType>vmx-21</vssd:VirtualSystemType>",
      "      </System>",
      "      <Item>",
      "        <rasd:AllocationUnits>hertz * 10^6</rasd:AllocationUnits>",
      "        <rasd:Description>Number of virtual CPUs</rasd:Description>",
      "        <rasd:ElementName>2 virtual CPU(s)</rasd:ElementName>",
      "        <rasd:InstanceID>1</rasd:InstanceID>",
      "        <rasd:ResourceType>3</rasd:ResourceType>",
      "        <rasd:VirtualQuantity>2</rasd:VirtualQuantity>",
      "      </Item>",
      "      <Item>",
      "        <rasd:AllocationUnits>byte * 2^20</rasd:AllocationUnits>",
      "        <rasd:Description>Memory Size</rasd:Description>",
      "        <rasd:ElementName>4096 MB of memory</rasd:ElementName>",
      "        <rasd:InstanceID>2</rasd:InstanceID>",
      "        <rasd:ResourceType>4</rasd:ResourceType>",
      "        <rasd:VirtualQuantity>4096</rasd:VirtualQuantity>",
      "      </Item>",
      "      <Item>",
      "        <rasd:AllocationUnits>byte * 2^30</rasd:AllocationUnits>",
      "        <rasd:Description>Virtual Disk</rasd:Description>",
      "        <rasd:ElementName>32 GB Virtual Disk</rasd:ElementName>",
      "        <rasd:HostResource>ovf:/disk/vmdisk1</rasd:HostResource>",
      "        <rasd:InstanceID>3</rasd:InstanceID>",
      "        <rasd:ResourceType>17</rasd:ResourceType>",
      "        <rasd:VirtualQuantity>32</rasd:VirtualQuantity>",
      "      </Item>",
      "      <Item>",
      "        <rasd:AutomaticAllocation>true</rasd:AutomaticAllocation>",
      "        <rasd:Connection>VM Network</rasd:Connection>",
      "        <rasd:ElementName>Ethernet adapter</rasd:ElementName>",
      "        <rasd:InstanceID>4</rasd:InstanceID>",
      "        <rasd:ResourceType>10</rasd:ResourceType>",
      "      </Item>",
      "      <vmw:ExtraConfig ovf:required=\"false\" vmw:key=\"vmx.allowNested\">TRUE</vmw:ExtraConfig>",
      "    </VirtualHardwareSection>",
      "  </VirtualSystem>",
      "</Envelope>",
      "OVFEOF",

      "# Create OVA (tar archive)",
      "tar -cf \"$${VMNAME}.ova\" \"$${VMNAME}.ovf\" \"$${VMNAME}-disk.vmdk\"",
      "echo \"=== OVA: $(ls -lh $${VMNAME}.ova | awk '{print $9, $5}')\"",
      "# Cleanup intermediate files",
      "rm -f \"$${VMNAME}.ovf\" \"$${VMNAME}-disk.vmdk\" \"$QCOW2\""
    ]
  }
}
