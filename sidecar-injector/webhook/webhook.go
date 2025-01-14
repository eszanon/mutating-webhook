package hook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/mutate,mutating=true,failurePolicy=fail,groups="",resources=pods,verbs=create;update,versions=v1,name=magalu.cloud.io

var log = logf.Log.WithName("sidecar-injector")

const floatingIP = "159.112.237.177"

// SidecarInjector annotates Pods
type sidecarInjector struct {
	Name          string
	Client        client.Client
	decoder       *admission.Decoder
	SidecarConfig *Config
}

type Config struct {
	Containers []corev1.Container `yaml:"containers"`
}

type ExtendedPod struct {
	corev1.Pod
}

// NewServiceAnnotator function creates a new instance of a service annotator
func NewSidecarInjector(name string, client client.Client, scheme *runtime.Scheme, config *Config) admission.Handler {

	return &sidecarInjector{
		Name:    name,
		Client:  client,
		decoder: admission.NewDecoder(scheme),

		SidecarConfig: config,
	}
}

func shoudInject(pod *corev1.Pod) bool {
	shouldInjectSidecar, err := strconv.ParseBool(pod.Annotations["inject-logging-sidecar"])

	if err != nil {
		shouldInjectSidecar = false
	}

	if shouldInjectSidecar {
		alreadyUpdated, err := strconv.ParseBool(pod.Annotations["logging-sidecar-added"])

		if err == nil && alreadyUpdated {
			shouldInjectSidecar = false
		}
	}

	// log.Info("Should Inject: ", shouldInjectSidecar)

	return shouldInjectSidecar
}

// SidecarInjector adds an annotation to every incoming pods.
func (si *sidecarInjector) Handle(ctx context.Context, req admission.Request) admission.Response {

	if req.RequestKind.Kind != "Service" {
		return admission.Allowed("Not a service")
	}

	switch req.Operation {
	case admissionv1.Create, admissionv1.Update:
		return si.handleCreateOrUpdate(ctx, req)
	case admissionv1.Delete:
		return si.handleDelete(ctx, req)
	}
	return admission.Allowed("Not a recognized operation")
}

func (si *sidecarInjector) handleCreateOrUpdate(ctx context.Context, req admission.Request) admission.Response {
	service := &corev1.Service{}
	err := si.decoder.Decode(req, service)

	if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		log.Info("Not a LoadBalancer service")
		return admission.Allowed("Not a LoadBalancer service")
	}

	if err != nil {
		log.Error(err, "Sidecar injector: cannot decode")
		return admission.Errored(http.StatusBadRequest, err)
	}

	if service.Name == "sample-service" {
		log.Info("Injecting loadBalancerIP on load balancer")
		service.Spec.LoadBalancerIP = floatingIP
	} else {
		log.Info("Not sample-service, denying request")
		return admission.Denied("No floating IP available!")
	}

	marshaledService, err := json.Marshal(service)

	if err != nil {
		log.Error(err, "Sidecar-Injector: cannot marshal")
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledService)
}

func (si *sidecarInjector) handleDelete(ctx context.Context, req admission.Request) admission.Response {
	//TODO: call VPC API here to delete floating IP
	msg := fmt.Sprintf("TODO: Delete FLOATING IP from MGC newtork API: %s", floatingIP)
	log.Info(msg)
	log.Info(fmt.Sprintf("Incoming delete request: %s", req.String()))
	return admission.Allowed(msg)
}
