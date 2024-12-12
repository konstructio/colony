package table

import (
	"fmt"
	"strings"

	tinkv1 "github.com/kubefirst/tink/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

type Column struct {
	Name  string
	Width int
	Align string // "left", "right", "center"
}

type TablePrinter struct {
	Columns []Column
}

func NewTablePrinter(columns []Column) *TablePrinter {
	return &TablePrinter{
		Columns: columns,
	}
}

func (t *TablePrinter) formatCell(value string, width int, align string) string {
	// Truncate if too long
	if len(value) > width {
		return value[:width-3] + "..."
	}

	// Pad according to alignment
	switch align {
	case "right":
		return strings.Repeat(" ", width-len(value)) + value
	case "center":
		leftPad := (width - len(value)) / 2
		rightPad := width - len(value) - leftPad
		return strings.Repeat(" ", leftPad) + value + strings.Repeat(" ", rightPad)
	default: // left align
		return value + strings.Repeat(" ", width-len(value))
	}
}

func (t *TablePrinter) calculateColumnWidths(rows []map[string]string) {
	for i, col := range t.Columns {
		maxWidth := len(col.Name)
		for _, row := range rows {
			if val, ok := row[col.Name]; ok {
				if len(val) > maxWidth {
					maxWidth = len(val)
				}
			}
		}
		t.Columns[i].Width = maxWidth + 2 // Add padding
	}
}

func (t *TablePrinter) PrintTable(rows []map[string]string) {
	t.calculateColumnWidths(rows)

	// Print header
	for _, col := range t.Columns {
		fmt.Print(t.formatCell(strings.ToUpper(col.Name), col.Width, "left"))
	}
	fmt.Println()

	// Print rows
	for _, row := range rows {
		for _, col := range t.Columns {
			value := row[col.Name]
			fmt.Print(t.formatCell(value, col.Width, col.Align))
		}
		fmt.Println()
	}
}

// Helper functions to convert objects to table rows

func HardwareToRow(hw *tinkv1.Hardware) map[string]string {
	return map[string]string{
		"name":     hw.Name,
		"hostname": hw.Annotations["inspection-status"],
		"ip":       hw.Spec.Interfaces[0].DHCP.IP.Address,
		"mac":      hw.Spec.Interfaces[0].DHCP.MAC,
		// "foo":      hw.Spec.BMCRef.Name,
		"status": string(hw.Status.State), // power
	}
}

func SecretToRow(secret *corev1.Secret) map[string]string {
	return map[string]string{
		"name":      secret.Name,
		"type":      string(secret.Type),
		"namespace": secret.Namespace,
		"age":       secret.CreationTimestamp.String(),
	}
}

// Example usage
func ExampleUsage() {
	// Hardware table example
	hwColumns := []Column{
		{Name: "name", Align: "left"},
		{Name: "hostname", Align: "left"},
		{Name: "ip", Align: "left"},
		{Name: "mac", Align: "left"},
		{Name: "status", Align: "left"},
	}

	hwPrinter := NewTablePrinter(hwColumns)

	// Example hardware data
	hwRows := []map[string]string{
		{
			"name":     "worker-1",
			"hostname": "bmc-1.example.com",
			"ip":       "192.168.1.100",
			"mac":      "00:11:22:33:44:55",
			"status":   "provisioning",
		},
		{
			"name":     "worker-2",
			"hostname": "bmc-2.example.com",
			"ip":       "192.168.1.101",
			"mac":      "00:11:22:33:44:56",
			"status":   "ready",
		},
	}

	hwPrinter.PrintTable(hwRows)

	// Secrets table example
	secretColumns := []Column{
		{Name: "name", Align: "left"},
		{Name: "namespace", Align: "left"},
		{Name: "type", Align: "left"},
		{Name: "age", Align: "right"},
	}

	secretPrinter := NewTablePrinter(secretColumns)

	// Example secrets data
	secretRows := []map[string]string{
		{
			"name":      "mysql-creds",
			"namespace": "default",
			"type":      "Opaque",
			"age":       "5d",
		},
		{
			"name":      "registry-pull-secret",
			"namespace": "kube-system",
			"type":      "kubernetes.io/dockerconfigjson",
			"age":       "30d",
		},
	}

	secretPrinter.PrintTable(secretRows)
}
