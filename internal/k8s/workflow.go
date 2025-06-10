package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	v1alpha1 "github.com/kubefirst/tink/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
)

//nolint:dupl
func (c *Client) returnWorkflowObject(ctx context.Context, gvr schema.GroupVersionResource, namespace string, timeoutSeconds int, opts metav1.ListOptions) (*v1alpha1.Workflow, error) {
	wf := &v1alpha1.Workflow{}

	err := wait.PollUntilContextTimeout(ctx, 15*time.Second, time.Duration(timeoutSeconds)*time.Second, true, func(ctx context.Context) (bool, error) {
		c.logger.Infof("getting workflow object with label %q", opts.LabelSelector)
		wfs, err := c.dynamic.Resource(gvr).Namespace(namespace).List(ctx, opts)
		if err != nil {
			// if we couldn't connect, ask to try again
			if isNetworkingError(err) {
				return false, nil
			}

			// if we got an error, return it
			return false, fmt.Errorf("error getting workflow object %q in namespace %q: %w", "matchLabel", namespace, err)
		}

		// if we couldn't find any workflow, ask to try again
		if len(wfs.Items) == 0 {
			return false, nil
		}

		err = runtime.DefaultUnstructuredConverter.FromUnstructured(wfs.Items[0].UnstructuredContent(), wf)
		if err != nil {
			return false, fmt.Errorf("error converting unstructured to workflow: %w", err)
		}

		// if we found a workflow, return it
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("error waiting for workflow: %w", err)
	}

	return wf, nil
}

func (c *Client) waitWorkflowComplete(ctx context.Context, gvr schema.GroupVersionResource, wfObj *v1alpha1.Workflow, timeoutSeconds int) (bool, error) {
	workflowName := wfObj.Name
	namespace := wfObj.Namespace

	wf := &v1alpha1.Workflow{}

	c.logger.Infof("waiting for workflow %q in namespace %q to be ready - this could take up to %d seconds", workflowName, namespace, timeoutSeconds)

	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, time.Duration(timeoutSeconds)*time.Second, true, func(ctx context.Context) (bool, error) {
		// Get the latest workflow object
		w, err := c.dynamic.Resource(gvr).Namespace(namespace).Get(ctx, workflowName, metav1.GetOptions{})
		if err != nil {
			// If we couldn't connect, retry
			if isNetworkingError(err) {
				c.logger.Warn("connection error, retrying: %s", err)
				return false, nil
			}

			return false, fmt.Errorf("error listing workflows: %w", err)
		}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(w.UnstructuredContent(), wf)
		if err != nil {
			return false, fmt.Errorf("error converting unstructured to workflow: %w", err)
		}

		jsonData, err := json.Marshal(wf.Status)
		if err != nil {
			log.Printf("Error marshaling: %v", err)
		}

		c.logger.Infof("%+v", string(jsonData))

		if len(wf.Status.Tasks) == 0 {
			return false, nil
		}
		if wf.Status.State == v1alpha1.WorkflowStatePending || wf.Status.State == v1alpha1.WorkflowStateRunning {
			return false, nil
		}

		if wf.Status.State == v1alpha1.WorkflowStateFailed {
			return true, fmt.Errorf("workflow %q in namespace %q failed: %+v", workflowName, namespace, wf.Status)
		}

		if wf.Status.State == v1alpha1.WorkflowStateSuccess {
			return true, nil
		}

		// workflow is not yet ready, continue polling
		return false, nil
	})
	if err != nil {
		return false, fmt.Errorf("the workflow %q in namespace %q was not ready within the timeout period: %w", workflowName, namespace, err)
	}

	return true, nil
}
