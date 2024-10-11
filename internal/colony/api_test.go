package colony

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

const (
	testValidToken = "super-duper-valid-token"
)

func TestAPI_ValidateApiKey(t *testing.T) {
	t.Run("valid API key", func(t *testing.T) {
		response := map[string]interface{}{
			"isValid": true,
		}

		mockServer := createServer(t, response, validateEndpoint)

		defer mockServer.Close()

		api := New(mockServer.URL, testValidToken)

		err := api.ValidateAPIKey(context.TODO())
		if err != nil {
			t.Fatalf("expected nil but got: %s", err)
		}
	})

	t.Run("invalid API key", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, `{"isValid": false}`)
		}))

		defer mockServer.Close()

		api := New(mockServer.URL, testValidToken)

		err := api.ValidateAPIKey(context.TODO())
		if !errors.Is(err, errInvalidKey) {
			t.Fatalf("expected %s, but got: %s", errInvalidKey, err)
		}
	})
}

func TestAPI_GetSystemTemplates(t *testing.T) {
	t.Run("valid response", func(t *testing.T) {
		response := []Template{{
			ID:             "k1",
			Name:           "name",
			Label:          "label",
			IsTinkTemplate: true,
			IsSystem:       true,
			Template:       "template_data",
		}}

		mockServer := createServer(t, response, templateEndpoint)

		defer mockServer.Close()

		api := New(mockServer.URL, testValidToken)

		templates, err := api.GetSystemTemplates(context.TODO())
		if err != nil {
			t.Fatalf("expected nil but got: %s", err)
		}

		if !reflect.DeepEqual(response, templates) {
			t.Fatalf("expected %#v got %#v", response, templates)
		}
	})

	t.Run("connection reset by peer", func(t *testing.T) {
		myListener, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			t.Fatalf("error creating listener %s", err)
		}
		address := myListener.Addr().String()

		go func() {
			for {
				con, err := myListener.Accept()
				if err != nil {
					t.Log(err)
				}
				con.Close()
			}
		}()

		api := New(address, testValidToken)

		_, err = api.GetSystemTemplates(context.TODO())
		if err == nil {
			t.Fatal("was expecting error but got none")
		}
	})
}

func createServer(t *testing.T, response interface{}, apiEndpoint string) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+apiEndpoint, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != fmt.Sprintf("Bearer %s", testValidToken) {
			t.Fatalf("expected to get a bearer token %s but got: %s", fmt.Sprintf("Bearer %s", testValidToken), r.Header.Get("Authorization"))
		}

		json.NewEncoder(w).Encode(response)
	})

	return httptest.NewServer(mux)
}

const templates = `[
  {
    "id": "128e31f2-5baa-4df1-8156-6cd28120f801",
    "created_at": "2024-04-02T17:54:33.456812Z",
    "updated_at": "2024-04-02T17:54:33.456812Z",
    "deleted_at": null,
    "name": "discovery",
    "label": "Discovery",
    "isTinkTemplate": true,
    "isSystem": true,
    "template": "apiVersion: \"tinkerbell.org/v1alpha1\"\nkind: Template\nmetadata:\n  name: discovery\n  namespace: tink-system\nspec:\n  data: |\n    version: \"0.1\"\n    name: discovery_spec\n    global_timeout: 1800\n    tasks:\n      - name: \"discovery-task\"\n        worker: \"{{.device_1}}\"\n        volumes:\n          - /lib/firmware:/lib/firmware:ro\n        actions:\n          - name: \"execute-discovery-script\"\n            image: ghcr.io/konstructio/discovery:v0.1.0\n            timeout: 180\n            environment:\n              K1_COLONY_HARDWARE_ID: \"{{.colony_hardware_id}}\"\n              COLONY_API_KEY: \"{{.colony_token}}\"  \n              CHROOT: y\n              COLONY_API_URL: \"{{.colony_api_url}}\"\n"
  },
  {
    "id": "1f278633-fc19-47e5-acf6-398caeab27ae",
    "created_at": "0001-01-01T00:00:00Z",
    "updated_at": "0001-01-01T00:00:00Z",
    "deleted_at": null,
    "name": "reboot",
    "label": "",
    "isTinkTemplate": true,
    "isSystem": true,
    "template": "apiVersion: \"tinkerbell.org/v1alpha1\"\nkind: Template\nmetadata:\n  name: reboot\n  namespace: tink-system\nspec:\n  data: |\n    version: \"0.1\"\n    name: discovery_spec\n    global_timeout: 1800\n    tasks:\n      - name: \"reboot-task\"\n        worker: \"{{.device_1}}\"\n        actions:\n          - name: \"reboot-machine\"\n            image: ghcr.io/jacobweinstock/waitdaemon:latest\n            timeout: 90\n            pid: host\n            command: [\"reboot\"]\n            environment:\n              IMAGE: alpine\n              WAIT_SECONDS: 10\n            volumes:\n              - /var/run/docker.sock:/var/run/docker.sock\n"
  },
  {
    "id": "dec20033-c049-4cea-b444-1b9046d3c5c6",
    "created_at": "2024-04-02T17:54:33.207143Z",
    "updated_at": "2024-04-02T17:54:33.207143Z",
    "deleted_at": null,
    "name": "ubuntu-focal",
    "label": "Ubuntu Focal",
    "isTinkTemplate": true,
    "isSystem": true,
    "template": "apiVersion: \"tinkerbell.org/v1alpha1\"\nkind: Template\nmetadata:\n  name: ubuntu-focal\n  namespace: tink-system\nspec:\n  data: |\n    version: \"0.1\"\n    name: ubuntu_focal\n    global_timeout: 9800\n    tasks:\n      - name: \"os-installation\"\n        worker: \"{{.device_1}}\"\n        volumes:\n          - /dev:/dev\n          - /dev/console:/dev/console\n          - /lib/firmware:/lib/firmware:ro\n        actions:\n          - name: \"stream-ubuntu-image\"\n            image: quay.io/tinkerbell-actions/image2disk:v1.0.0\n            timeout: 9600\n            environment:\n              DEST_DISK: {{ .disk }}\n              IMG_URL: \"http://10.0.10.2:8080/jammy-server-cloudimg-amd64.raw.gz\"\n              COMPRESSED: true\n          - name: \"grow-partition\"\n            image: quay.io/tinkerbell-actions/cexec:v1.0.0\n            timeout: 90\n            environment:\n              BLOCK_DEVICE: {{ .disk }}{{.block_partition}}\n              FS_TYPE: ext4\n              CHROOT: y\n              DEFAULT_INTERPRETER: \"/bin/sh -c\"\n              CMD_LINE: \"growpart {{ .disk }} 1 && resize2fs {{ .disk }}{{.block_partition}}\"\n          - name: \"install-openssl\"\n            image: quay.io/tinkerbell-actions/cexec:v1.0.0\n            timeout: 90\n            environment:\n              BLOCK_DEVICE: {{ .disk }}{{.block_partition}}\n              FS_TYPE: ext4\n              CHROOT: y\n              DEFAULT_INTERPRETER: \"/bin/sh -c\"\n              CMD_LINE: \"apt -y update && apt -y install openssl\"\n          - name: \"create-user\"\n            image: quay.io/tinkerbell-actions/cexec:v1.0.0\n            timeout: 90\n            environment:\n              BLOCK_DEVICE: {{ .disk }}{{.block_partition}}\n              FS_TYPE: ext4\n              CHROOT: y\n              DEFAULT_INTERPRETER: \"/bin/sh -c\"\n              CMD_LINE: \"useradd -p $(openssl passwd -1 tink) -s /bin/bash -d /home/tink/ -m -G sudo tink\"\n          - name: \"enable-ssh\"\n            image: quay.io/tinkerbell-actions/cexec:v1.0.0\n            timeout: 90\n            environment:\n              BLOCK_DEVICE: {{ .disk }}{{.block_partition}}\n              FS_TYPE: ext4\n              CHROOT: y\n              DEFAULT_INTERPRETER: \"/bin/sh -c\"\n              CMD_LINE: \"ssh-keygen -A; systemctl enable ssh.service; sed -i 's/^PasswordAuthentication no/PasswordAuthentication yes/g' /etc/ssh/sshd_config\"\n          - name: \"disable-apparmor\"\n            image: quay.io/tinkerbell-actions/cexec:v1.0.0\n            timeout: 90\n            environment:\n              BLOCK_DEVICE: {{ .disk }}{{.block_partition}}\n              FS_TYPE: ext4\n              CHROOT: y\n              DEFAULT_INTERPRETER: \"/bin/sh -c\"\n              CMD_LINE: \"systemctl disable apparmor; systemctl disable snapd\"\n          - name: \"write-netplan\"\n            image: quay.io/tinkerbell-actions/writefile:v1.0.0\n            timeout: 90\n            environment:\n              DEST_DISK: {{ .disk }}{{.block_partition}}\n              FS_TYPE: ext4\n              DEST_PATH: /etc/netplan/config.yaml\n              CONTENTS: |\n                network:\n                  version: 2\n                  renderer: networkd\n                  ethernets:\n                    id0:\n                      match:\n                        name: en*\n                      dhcp4: true\n              UID: 0\n              GID: 0\n              MODE: 0644\n              DIRMODE: 0755\n          - name: \"kexec\"\n            image: ghcr.io/jacobweinstock/waitdaemon:latest\n            timeout: 90\n            pid: host\n            environment:\n              BLOCK_DEVICE: {{ formatPartition ( .disk ) 1 }}\n              FS_TYPE: ext4\n              IMAGE: quay.io/tinkerbell-actions/kexec:v1.0.0\n              WAIT_SECONDS: 10\n            volumes:\n              - /var/run/docker.sock:/var/run/docker.sock"
  },
  {
    "id": "64e32576-7994-472c-98d8-979724c11e69",
    "created_at": "0001-01-01T00:00:00Z",
    "updated_at": "0001-01-01T00:00:00Z",
    "deleted_at": null,
    "name": "ubuntu-focal-k3s-join",
    "label": "Ubuntu K3S Join",
    "isTinkTemplate": true,
    "isSystem": true,
    "template": "apiVersion: \"tinkerbell.org/v1alpha1\"\nkind: Template\nmetadata:\n  name: ubuntu-focal-k3s-join\nspec:\n  data: |\n    version: \"0.1\"\n    name: ubuntu_Focal\n    global_timeout: 1800\n    tasks:\n      - name: \"os-installation\"\n        worker: \"{{.device_1}}\"\n        volumes:\n          - /dev:/dev\n          - /dev/console:/dev/console\n          - /lib/firmware:/lib/firmware:ro\n        actions:\n          - name: \"stream-ubuntu-image\"\n            image: quay.io/tinkerbell-actions/image2disk:v1.0.0\n            timeout: 600\n            environment:\n              DEST_DISK: {{ .disk }}\n              IMG_URL: \"http://10.0.10.2:8080/jammy-server-cloudimg-amd64.raw.gz\"\n              COMPRESSED: true\n          - name: \"grow-partition\"\n            image: quay.io/tinkerbell-actions/cexec:v1.0.0\n            timeout: 90\n            environment:\n              BLOCK_DEVICE: {{ .disk }}{{.block_partition}}\n              FS_TYPE: ext4\n              CHROOT: y\n              DEFAULT_INTERPRETER: \"/bin/sh -c\"\n              CMD_LINE: \"growpart {{ .disk }} 1 && resize2fs {{ .disk }}{{.block_partition}}\"\n          - name: \"install-openssl\"\n            image: quay.io/tinkerbell-actions/cexec:v1.0.0\n            timeout: 90\n            environment:\n              BLOCK_DEVICE: {{ .disk }}{{.block_partition}}\n              FS_TYPE: ext4\n              CHROOT: y\n              DEFAULT_INTERPRETER: \"/bin/sh -c\"\n              CMD_LINE: \"apt -y update && apt -y install openssl\"\n          - name: \"create-user\"\n            image: quay.io/tinkerbell-actions/cexec:v1.0.0\n            timeout: 90\n            environment:\n              BLOCK_DEVICE: {{ .disk }}{{.block_partition}}\n              FS_TYPE: ext4\n              CHROOT: y\n              DEFAULT_INTERPRETER: \"/bin/sh -c\"\n              CMD_LINE: \"useradd -p $(openssl passwd -1 tink) -s /bin/bash -d /home/tink/ -m -G sudo tink\"\n          - name: \"enable-ssh\"\n            image: quay.io/tinkerbell-actions/cexec:v1.0.0\n            timeout: 90\n            environment:\n              BLOCK_DEVICE: {{ .disk }}{{.block_partition}}\n              FS_TYPE: ext4\n              CHROOT: y\n              DEFAULT_INTERPRETER: \"/bin/sh -c\"\n              CMD_LINE: \"ssh-keygen -A; systemctl enable ssh.service; echo 'PasswordAuthentication yes' > /etc/ssh/sshd_config.d/60-cloudimg-settings.conf\"\n          - name: \"disable-apparmor\"\n            image: quay.io/tinkerbell-actions/cexec:v1.0.0\n            timeout: 90\n            environment:\n              BLOCK_DEVICE: {{ .disk }}{{.block_partition}}\n              FS_TYPE: ext4\n              CHROOT: y\n              DEFAULT_INTERPRETER: \"/bin/sh -c\"\n              CMD_LINE: \"systemctl disable apparmor; systemctl disable snapd\"\n          - name: \"write-netplan\"\n            image: quay.io/tinkerbell-actions/writefile:v1.0.0\n            timeout: 90\n            environment:\n              DEST_DISK: {{ .disk }}{{.block_partition}}\n              FS_TYPE: ext4\n              DEST_PATH: /etc/netplan/config.yaml\n              CONTENTS: |\n                network:\n                  version: 2\n                  renderer: networkd\n                  ethernets:\n                    id0:\n                      match:\n                        name: en*\n                      dhcp4: false\n                      addresses:\n                        - {{ .static_ip }}\n                      routes:\n                        - to: default\n                          via: {{ .gateway }}                      \n                      nameservers:\n                        addresses:\n                          - 8.8.8.8\n                          - 8.8.4.4\n              UID: 0\n              GID: 0\n              MODE: 0644\n              DIRMODE: 0755\n          - name: \"k3s-installation\"\n            image: quay.io/tinkerbell-actions/writefile:v1.0.0\n            timeout: 120\n            environment:\n              DEST_DISK: {{ .disk }}{{.block_partition}}\n              FS_TYPE: ext4\n              DEST_PATH: \"/var/lib/cloud/seed/nocloud-net/user-data\"\n              CONTENTS: |\n                #cloud-config\n                runcmd:\n                  - apt -y update\n                  - apt -y install curl\n                  - |\n                    role=\"{{ .role }}\"\n                    if [ \"$role\" = \"server\" ]; then\n                      echo \"joining the cluster as a server\"\n                      curl -sfL https://get.k3s.io | K3S_TOKEN={{ .k3s_token }} sh -s - server --server https://{{ .k3s_server_ip }}:6443\n                    else\n                      echo \"joining the cluster as an agent\"\n                      curl -sfL https://get.k3s.io | K3S_URL=https://{{ .k3s_server_ip }}:6443 K3S_TOKEN={{ .k3s_token }} sh -\n                    fi\n                  - |\n                    # Report final status of cloud init\n                    curl -L -o colony-scout.tar.gz https://assets.kubefirst.com/colony/colony-scout_.0.0.10-rc8_Linux_x86_64.tar.gz\n                    tar -xzvf colony-scout.tar.gz\n                    chmod +x colony-scout\n\n                    output=$(./colony-scout report --validate=k8s,cloud-init \\\n                      --type=agent \\\n                      --token={{ .colony_token }} \\\n                      --colony-api={{ .colony_api_url }} \\\n                      --cluster-id={{ .colony_cluster_id }} \\\n                      --workflow-id={{ .colony_workflow_id }} \\\n                      --hardware-id={{ .colony_hardware_id }} \\\n                      --host-ip-port={{ .static_ip }} 2>&1)\n                    echo \"Colony scout output: $output\"\n\n              UID: 0\n              GID: 0\n              MODE: 0644\n              DIRMODE: 0755\n          - name: \"write-meta-data\"\n            image: quay.io/tinkerbell-actions/writefile:v1.0.0\n            timeout: 90\n            environment:\n              DEST_DISK: {{ .disk }}{{.block_partition}}\n              FS_TYPE: ext4\n              DEST_PATH: \"/var/lib/cloud/seed/nocloud-net/meta-data\"\n              CONTENTS: |\n                instance-id: {{ .device_1 }}\n                local-hostname: {{ .hostname }}\n              UID: 0\n              GID: 0\n              MODE: 0644\n              DIRMODE: 0755\n          - name: \"kexec\"\n            image: ghcr.io/jacobweinstock/waitdaemon:latest\n            timeout: 90\n            pid: host\n            environment:\n              BLOCK_DEVICE: {{ formatPartition ( .disk ) 1 }}\n              FS_TYPE: ext4\n              IMAGE: quay.io/tinkerbell-actions/kexec:v1.0.0\n              WAIT_SECONDS: 10\n            volumes:\n              - /var/run/docker.sock:/var/run/docker.sock\n"
  },
  {
    "id": "1423722f-7d17-4e58-8d5b-3e820a6b9e74",
    "created_at": "0001-01-01T00:00:00Z",
    "updated_at": "0001-01-01T00:00:00Z",
    "deleted_at": null,
    "name": "ubuntu-focal-k3s-server",
    "label": "Ubuntu K3S Server",
    "isTinkTemplate": true,
    "isSystem": true,
    "template": "apiVersion: \"tinkerbell.org/v1alpha1\"\nkind: Template\nmetadata:\n  name: ubuntu-focal-k3s-server\nspec:\n  data: |\n    version: \"0.1\"\n    name: ubuntu_Focal\n    global_timeout: 1800\n    tasks:\n      - name: \"os-installation\"\n        worker: \"{{.device_1}}\"\n        volumes:\n          - /dev:/dev\n          - /dev/console:/dev/console\n          - /lib/firmware:/lib/firmware:ro\n        actions:\n          - name: \"stream-ubuntu-image\"\n            image: quay.io/tinkerbell-actions/image2disk:v1.0.0\n            timeout: 600\n            environment:\n              DEST_DISK: {{ .disk }}\n              IMG_URL: \"http://10.0.10.2:8080/jammy-server-cloudimg-amd64.raw.gz\"\n              COMPRESSED: true\n          - name: \"grow-partition\"\n            image: quay.io/tinkerbell-actions/cexec:v1.0.0\n            timeout: 90\n            environment:\n              BLOCK_DEVICE: {{ .disk }}{{.block_partition}}\n              FS_TYPE: ext4\n              CHROOT: y\n              DEFAULT_INTERPRETER: \"/bin/sh -c\"\n              CMD_LINE: \"growpart {{ .disk }} 1 && resize2fs {{ .disk }}{{.block_partition}}\"\n          - name: \"install-openssl\"\n            image: quay.io/tinkerbell-actions/cexec:v1.0.0\n            timeout: 90\n            environment:\n              BLOCK_DEVICE: {{ .disk }}{{.block_partition}}\n              FS_TYPE: ext4\n              CHROOT: y\n              DEFAULT_INTERPRETER: \"/bin/sh -c\"\n              CMD_LINE: \"apt -y update && apt -y install openssl\"\n          - name: \"create-user\"\n            image: quay.io/tinkerbell-actions/cexec:v1.0.0\n            timeout: 90\n            environment:\n              BLOCK_DEVICE: {{ .disk }}{{.block_partition}}\n              FS_TYPE: ext4\n              CHROOT: y\n              DEFAULT_INTERPRETER: \"/bin/sh -c\"\n              CMD_LINE: \"useradd -p $(openssl passwd -1 tink) -s /bin/bash -d /home/tink/ -m -G sudo tink\"\n          - name: \"enable-ssh\"\n            image: quay.io/tinkerbell-actions/cexec:v1.0.0\n            timeout: 90\n            environment:\n              BLOCK_DEVICE: {{ .disk }}{{.block_partition}}\n              FS_TYPE: ext4\n              CHROOT: y\n              DEFAULT_INTERPRETER: \"/bin/sh -c\"\n              CMD_LINE: \"ssh-keygen -A; systemctl enable ssh.service; echo 'PasswordAuthentication yes' > /etc/ssh/sshd_config.d/60-cloudimg-settings.conf\"\n          - name: \"disable-apparmor\"\n            image: quay.io/tinkerbell-actions/cexec:v1.0.0\n            timeout: 90\n            environment:\n              BLOCK_DEVICE: {{ .disk }}{{.block_partition}}\n              FS_TYPE: ext4\n              CHROOT: y\n              DEFAULT_INTERPRETER: \"/bin/sh -c\"\n              CMD_LINE: \"systemctl disable apparmor; systemctl disable snapd\"\n          - name: \"write-netplan\"\n            image: quay.io/tinkerbell-actions/writefile:v1.0.0\n            timeout: 90\n            environment:\n              DEST_DISK: {{ .disk }}{{.block_partition}}\n              FS_TYPE: ext4\n              DEST_PATH: /etc/netplan/config.yaml\n              CONTENTS: |\n                network:\n                  version: 2\n                  renderer: networkd\n                  ethernets:\n                    id0:\n                      match:\n                        name: en*\n                      dhcp4: false\n                      addresses:\n                        - {{ .static_ip }}\n                      routes:\n                        - to: default\n                          via: {{ .gateway }}                      \n                      nameservers:\n                        addresses:\n                          - 8.8.8.8\n                          - 8.8.4.4\n              UID: 0\n              GID: 0\n              MODE: 0644\n              DIRMODE: 0755\n          - name: \"k3s-installation\"\n            image: quay.io/tinkerbell-actions/writefile:v1.0.0\n            timeout: 120\n            environment:\n              DEST_DISK: {{ .disk }}{{.block_partition}}\n              FS_TYPE: ext4\n              DEST_PATH: \"/var/lib/cloud/seed/nocloud-net/user-data\"\n              CONTENTS: |\n                #cloud-config\n                runcmd:\n                  - apt -y update\n                  - apt -y install curl\n                  - |\n                    multi_master=\"{{ .multi_master }}\"\n                    if [ \"$multi_master\" = \"true\" ]; then\n                      echo \"Multi master cluster\"\n                      curl -sfL https://get.k3s.io | sh -s server --cluster-init --tls-san={{ .extra_sans }}\n                    else\n                      echo \"Single master cluster\"\n                      curl -sfL https://get.k3s.io | sh -s - --tls-san={{ .extra_sans }}\n                    fi\n                  - |\n                    # Report final status of cloud init\n                    curl -L -o colony-scout.tar.gz https://assets.kubefirst.com/colony/colony-scout_.0.0.10-rc8_Linux_x86_64.tar.gz\n                    tar -xzvf colony-scout.tar.gz\n                    chmod +x colony-scout\n    \n                    K3S_TOKEN=$(cat /var/lib/rancher/k3s/server/node-token)\n    \n                    output=$(./colony-scout report --validate=k8s,cloud-init \\\n                      --type=server \\\n                      --token={{ .colony_token }} \\\n                      --colony-api={{ .colony_api_url }} \\\n                      --cluster-id={{ .colony_cluster_id }} \\\n                      --workflow-id={{ .colony_workflow_id }} \\\n                      --hardware-id={{ .colony_hardware_id }} \\\n                      --host-ip-port={{ .static_ip }}:6443 \\\n                      --kubeconfig=/etc/rancher/k3s/k3s.yaml \\\n                      --k3s-token=$K3S_TOKEN 2>&1)\n                    \n                    echo \"Colony scout output: $output\"\n                    \n              UID: 0\n              GID: 0\n              MODE: 0644\n              DIRMODE: 0755\n          - name: \"write-meta-data\"\n            image: quay.io/tinkerbell-actions/writefile:v1.0.0\n            timeout: 90\n            environment:\n              DEST_DISK: {{ .disk }}{{.block_partition}}\n              FS_TYPE: ext4\n              DEST_PATH: \"/var/lib/cloud/seed/nocloud-net/meta-data\"\n              CONTENTS: |\n                instance-id: {{ .device_1 }}\n                local-hostname: {{ .hostname }}\n              UID: 0\n              GID: 0\n              MODE: 0644\n              DIRMODE: 0755\n          - name: \"kexec\"\n            image: ghcr.io/jacobweinstock/waitdaemon:latest\n            timeout: 90\n            pid: host\n            environment:\n              BLOCK_DEVICE: {{ formatPartition ( .disk ) 1 }}\n              FS_TYPE: ext4\n              IMAGE: quay.io/tinkerbell-actions/kexec:v1.0.0\n              WAIT_SECONDS: 10\n            volumes:\n              - /var/run/docker.sock:/var/run/docker.sock\n"
  }
]`

func TestGetSystemTemplates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(templates))
	}))

	api := New(srv.URL, testValidToken)

	templates, err := api.GetSystemTemplates(context.Background())
	if err != nil {
		t.Fatalf("expected nil but got: %s", err)
	}

	if len(templates) != 5 {
		t.Fatalf("expected 5 templates but got: %d", len(templates))
	}
}
