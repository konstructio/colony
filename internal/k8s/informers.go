package k8s

import (
	"context"
	"strings"

	"github.com/konstructio/colony/internal/constants"
	"github.com/kubefirst/tink/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

func (c *Client) HardwareInformer(ctx context.Context, ipmiIP string, hardwareChan chan *v1alpha1.Hardware) error {
	// Create a new informer for the hardware resource
	resource := v1alpha1.GroupVersion.WithResource("hardware")
	factory := dynamicinformer.NewDynamicSharedInformerFactory(c.dynamic, 0)
	informer := factory.ForResource(resource).Informer()

	// Add event handlers to the informer
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			hw := &v1alpha1.Hardware{}
			unst := obj.(*unstructured.Unstructured)
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unst.Object, hw); err != nil {
				c.logger.Errorf("Error converting unstructured to hardware: %v\n", err)
				return
			}

			c.logger.Infof("Hardware %q created by - id: %q \n", hw.Name, hw.ObjectMeta.UID)

			err := c.SecretAddLabel(ctx, strings.ReplaceAll(ipmiIP, ".", "-"), constants.ColonyNamespace, "colony.konstruct.io/hardware-id", hw.Name)
			if err != nil {
				c.logger.Errorf("Error adding label to secret: %v\n", err)
				return
			}
			c.logger.Infof("added label to ipmi secret: %s\n", hw.Name)
			// Send the hardware object through the channel
			hardwareChan <- hw
		},
	})

	informer.Run(ctx.Done())
	return nil
}
