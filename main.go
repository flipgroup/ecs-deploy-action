package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
)

const ecsPollInterval = 3 * time.Second

func exitUsage() {
	fmt.Println("deploy-ecs-task cluster service-name container-name=image:version")
	os.Exit(1)
}

func main() {
	if len(os.Args) < 3 {
		exitUsage()
	}

	cluster := os.Args[1]
	serviceName := os.Args[2]

	replaces := map[string]string{}
	for _, s := range strings.Split(os.Args[3], ",") {
		s = strings.TrimSpace(s)
		parts := strings.Split(s, "=")
		if len(parts) != 2 {
			fmt.Println("must specify image replace in format container-name=new-docker-image:version")
			exitUsage()
		}
		replaces[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}

	sess := session.Must(session.NewSession())
	svc := ecs.New(sess)

	taskDefinition := getCurrentTaskDefinition(svc, cluster, serviceName)

	for _, container := range taskDefinition.ContainerDefinitions {
		for targetName, targetImage := range replaces {
			if *container.Name != targetName {
				continue
			}

			container.SetImage(targetImage)
		}
	}

	newTaskArn := uploadTask(svc, taskDefinition)

	updateService(svc, cluster, serviceName, newTaskArn)
}

func getCurrentTaskDefinition(svc *ecs.ECS, cluster string, serviceName string) *ecs.TaskDefinition {
	serviceResp, err := svc.DescribeServices(&ecs.DescribeServicesInput{
		Cluster:  &cluster,
		Services: []*string{&serviceName},
	})
	if err != nil {
		log.Fatal("failed to find existing service:", err)
	}

	if len(serviceResp.Failures) > 0 {
		log.Fatal("failed to find existing service:", serviceResp.Failures[0])
	}

	if len(serviceResp.Services) == 0 {
		log.Fatal("service not found")
	}

	taskDef, err := svc.DescribeTaskDefinition(&ecs.DescribeTaskDefinitionInput{
		TaskDefinition: serviceResp.Services[0].TaskDefinition,
	})
	if err != nil {
		log.Fatal("failed to find existing task definition:", err)
	}

	return taskDef.TaskDefinition
}

func uploadTask(svc *ecs.ECS, def *ecs.TaskDefinition) string {

	// stupid hack to move data between the different types
	taskJson, err := json.Marshal(def)
	if err != nil {
		panic(err)
	}
	var input ecs.RegisterTaskDefinitionInput
	if err = json.Unmarshal(taskJson, &input); err != nil {
		panic(err)
	}

	if err := input.Validate(); err != nil {
		log.Fatal("failed to validate new task definition", err)
	}

	log.Printf("Registering a task for %s\n", *input.Family)
	resp, err := svc.RegisterTaskDefinition(&input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to register task definition: %s\n", err)
		os.Exit(1)
	}

	log.Printf("Created %s\n", *resp.TaskDefinition.TaskDefinitionArn)

	return *resp.TaskDefinition.TaskDefinitionArn
}

func updateService(svc *ecs.ECS, cluster, service, taskDefinition string) {
	log.Printf("Updating service %s\n", service)

	_, err := svc.UpdateService(&ecs.UpdateServiceInput{
		Cluster:        &cluster,
		Service:        &service,
		TaskDefinition: &taskDefinition,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to update service %s on cluster %s: %s\n", service, cluster, err)
		os.Exit(1)
	}

	pollUntilTaskDeployed(svc, cluster, service, taskDefinition)
}

func getService(svc *ecs.ECS, service, cluster string) (*ecs.Service, error) {
	resp, err := svc.DescribeServices(&ecs.DescribeServicesInput{
		Services: []*string{aws.String(service)},
		Cluster:  aws.String(cluster),
	})
	if err != nil {
		return nil, err
	}

	if len(resp.Failures) > 0 {
		return nil, errors.New(*resp.Failures[0].Reason)
	}

	if len(resp.Services) != 1 {
		return nil, errors.New("multiple services with the same name")
	}

	return resp.Services[0], nil
}

func pollUntilTaskDeployed(svc *ecs.ECS, service string, cluster string, task string) {
	lastSeen := time.Now().Add(-1 * time.Minute)

	for {
		service, err := getService(svc, cluster, service)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to get service: %s\n", err)
			os.Exit(1)
		}

		for i := len(service.Events) - 1; i >= 0; i-- {
			event := service.Events[i]
			if event.CreatedAt.After(lastSeen) {
				log.Println(*event.Message)
				lastSeen = *event.CreatedAt
			}
		}

		if len(service.Deployments) == 1 && *service.Deployments[0].TaskDefinition == task {
			return
		}

		time.Sleep(ecsPollInterval)
	}
}
