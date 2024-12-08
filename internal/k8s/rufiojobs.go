package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	rufiov1 "github.com/tinkerbell/rufio/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
)

func (c *Client) returnRufioJobObject(ctx context.Context, gvr schema.GroupVersionResource, matchLabel, matchLabelValue, namespace string, timeoutSeconds int) (*rufiov1.Job, error) {

	job := &rufiov1.Job{}

	err := wait.PollUntilContextTimeout(ctx, 15*time.Second, time.Duration(timeoutSeconds)*time.Second, true, func(ctx context.Context) (bool, error) {
		c.logger.Infof("getting job object with label %q", fmt.Sprintf("%s=%s", matchLabel, matchLabelValue))
		jobs, err := c.dynamic.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", matchLabel, matchLabelValue),
		})
		if err != nil {
			// if we couldn't connect, ask to try again
			if isNetworkingError(err) {
				return false, nil
			}

			// if we got an error, return it
			return false, fmt.Errorf("error getting job object %q in namespace %q: %w", "matchLabel", namespace, err)
		}

		// if we couldn't find any job, ask to try again
		if len(jobs.Items) == 0 {
			return false, nil
		}

		err = runtime.DefaultUnstructuredConverter.FromUnstructured(jobs.Items[0].UnstructuredContent(), job)
		if err != nil {
			return false, fmt.Errorf("error converting unstructured to job: %w", err)
		}

		// if we found a job, return it
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("error waiting for job: %w", err)
	}

	return job, nil
}

func (c *Client) waitForJobComplete(ctx context.Context, gvr schema.GroupVersionResource, jobObj *rufiov1.Job, timeoutSeconds int) (bool, error) {
	jobName := jobObj.Name
	namespace := jobObj.Namespace

	job := &rufiov1.Job{}

	c.logger.Infof("waiting for job %q in namespace %q to be ready - this could take up to %d seconds", jobName, namespace, timeoutSeconds)

	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, time.Duration(timeoutSeconds)*time.Second, true, func(ctx context.Context) (bool, error) {
		// Get the latest Machine object
		j, err := c.dynamic.Resource(gvr).Namespace(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
		if err != nil {
			// If we couldn't connect, retry
			if isNetworkingError(err) {
				c.logger.Warn("connection error, retrying: %s", err)
				return false, nil
			}

			return false, fmt.Errorf("error listing machines: %w", err)
		}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(j.UnstructuredContent(), job)
		if err != nil {
			return false, fmt.Errorf("error converting unstructured to job: %w", err)
		}

		jsonData, err := json.Marshal(job.Status)
		if err != nil {
			log.Printf("Error marshaling: %v", err)
		}

		c.logger.Infof("debug: %+v", string(jsonData))
		if len(job.Status.Conditions) > 1 {
			return false, nil
		}

		// Check the status and conditions of the job
		if job.Status.Conditions[0].Status == "True" && job.Status.Conditions[0].Type == rufiov1.JobCompleted {
			return true, nil
		}

		// Machine is not yet ready, continue polling
		return false, nil
	})
	if err != nil {
		return false, fmt.Errorf("the job %q in namespace %q was not ready within the timeout period: %w", jobName, namespace, err)
	}

	return true, nil
}
