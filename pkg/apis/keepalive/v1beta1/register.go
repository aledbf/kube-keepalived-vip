package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// VirtualIP is a keepalived CRD specificiation for virtual IP addresses
type VirtualIP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   VirtualIPSpec `json:"spec"`
	Status Status        `json:"status"`
}

// VirtualIPSpec defines the spec of the CRD
type VirtualIPSpec struct {
	// IP virtual IP to use
	IP string `json:"ip"`
	// Port of the virtual
	Port int `json:"port"`

	// Interface interface where the virtual IP should be configured
	// The default value is eth0
	Interface *string `json:"interface,omitempty"`

	// ProxyProtocol indicates if proxy-protocol is enabled in the service being exposed
	ProxyProtocol bool `json:"proxyProtocol"`
	//Protocol defines the protocol of the service. Valid values are TCP and UDP
	Protocol corev1.Protocol `json:"protocol"`
	// ServiceReference defines a reference to the service to expose
	ServiceReference ServiceReference `json:"serviceReference"`

	// VirtualRouterID arbitrary unique number from 0 to 255.
	// Used to differentiate multiple instances of vrrpd running on the same NIC (and hence same socket)
	VirtualRouterID int `json:"VirtualRouterID,omitempty"`
	// Priority for electing MASTER, highest priority wins
	// To be MASTER, make this 50 more than on other machines.
	Priority int `json:"priority,omitempty"`
	// DelayLoop delay timer for checker polling
	// Default: 5
	DelayLoop *int `json:"delayLoop,omitempty"`
	// LVSScheduler LVS scheduler (rr|wrr|lc|wlc|lblc|sh|mh|dh|fo|ovf|lblcr|sed|nq)
	// Default: wlc
	LVSScheduler *string `json:"lvsScheduler,omitempty"`
	// LVSMethod default LVS forwarding method (NAT|DR)
	// Default: NAT
	LVSMethod *string `json:"lvsMethod,omitempty"`

	// UseUnicast defines if unicast should be used instead of multicast (default) to publish vrrp packets
	UseUnicast bool `json:"useUnicast,omitempty"`
	// Notify defines a script for ANY state transition.
	Notify *string `json:"notify,omitempty"`
	// ConfigurationSnippet defines additional keepalived configuration
	ConfigurationSnippet *string `json:"configurationSnippet,omitempty"`
}

// Status reports the current state of the VirtualIP
type Status struct {
	CurrentStatus string `json:"currentStatus"`
	Description   string `json:"description"`
}

// ServiceReference holds a reference to Service.legacy.k8s.io
type ServiceReference struct {
	// Namespace is the namespace of the service
	Namespace string `json:"namespace,omitempty"`
	// Name is the name of the service
	Name string `json:"name,omitempty"`
	// Port of the service
	Port intstr.IntOrString `json:"port,omitempty"`
}
