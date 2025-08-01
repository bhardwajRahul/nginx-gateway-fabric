package state_test

import (
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	apiv1 "k8s.io/api/core/v1"
	discoveryV1 "k8s.io/api/discovery/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	"sigs.k8s.io/gateway-api/apis/v1alpha3"
	"sigs.k8s.io/gateway-api/apis/v1beta1"

	ngfAPIv1alpha1 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha1"
	ngfAPIv1alpha2 "github.com/nginx/nginx-gateway-fabric/v2/apis/v1alpha2"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/conditions"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/validation/validationfakes"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/controller/index"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kinds"
	ngftypes "github.com/nginx/nginx-gateway-fabric/v2/internal/framework/types"
)

const (
	controllerName    = "my.controller"
	gcName            = "test-class"
	httpListenerName  = "listener-80-1"
	httpsListenerName = "listener-443-1"
	tlsListenerName   = "listener-8443-1"
)

func createHTTPRoute(
	name string,
	gateway string,
	hostname string,
	backendRefs ...v1.HTTPBackendRef,
) *v1.HTTPRoute {
	return &v1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  "test",
			Name:       name,
			Generation: 1,
		},
		Spec: v1.HTTPRouteSpec{
			CommonRouteSpec: v1.CommonRouteSpec{
				ParentRefs: []v1.ParentReference{
					{
						Namespace:   (*v1.Namespace)(helpers.GetPointer("test")),
						Name:        v1.ObjectName(gateway),
						SectionName: (*v1.SectionName)(helpers.GetPointer(httpListenerName)),
					},
					{
						Namespace:   (*v1.Namespace)(helpers.GetPointer("test")),
						Name:        v1.ObjectName(gateway),
						SectionName: (*v1.SectionName)(helpers.GetPointer(httpsListenerName)),
					},
				},
			},
			Hostnames: []v1.Hostname{
				v1.Hostname(hostname),
			},
			Rules: []v1.HTTPRouteRule{
				{
					Matches: []v1.HTTPRouteMatch{
						{
							Path: &v1.HTTPPathMatch{
								Type:  helpers.GetPointer(v1.PathMatchPathPrefix),
								Value: helpers.GetPointer("/"),
							},
						},
					},
					BackendRefs: backendRefs,
				},
			},
		},
	}
}

func createGRPCRoute(
	name string,
	gateway string,
	hostname string,
	backendRefs ...v1.GRPCBackendRef,
) *v1.GRPCRoute {
	return &v1.GRPCRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  "test",
			Name:       name,
			Generation: 1,
		},
		Spec: v1.GRPCRouteSpec{
			CommonRouteSpec: v1.CommonRouteSpec{
				ParentRefs: []v1.ParentReference{
					{
						Namespace:   (*v1.Namespace)(helpers.GetPointer("test")),
						Name:        v1.ObjectName(gateway),
						SectionName: (*v1.SectionName)(helpers.GetPointer(httpListenerName)),
					},
					{
						Namespace:   (*v1.Namespace)(helpers.GetPointer("test")),
						Name:        v1.ObjectName(gateway),
						SectionName: (*v1.SectionName)(helpers.GetPointer(httpsListenerName)),
					},
				},
			},
			Hostnames: []v1.Hostname{
				v1.Hostname(hostname),
			},
			Rules: []v1.GRPCRouteRule{
				{
					Matches: []v1.GRPCRouteMatch{
						{
							Method: &v1.GRPCMethodMatch{
								Type:    helpers.GetPointer(v1.GRPCMethodMatchExact),
								Service: helpers.GetPointer("my-svc"),
								Method:  helpers.GetPointer("Hello"),
							},
						},
					},
					BackendRefs: backendRefs,
				},
			},
		},
	}
}

func createTLSRoute(name, gateway, hostname string, backendRefs ...v1.BackendRef) *v1alpha2.TLSRoute {
	return &v1alpha2.TLSRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  "test",
			Name:       name,
			Generation: 1,
		},
		Spec: v1alpha2.TLSRouteSpec{
			CommonRouteSpec: v1.CommonRouteSpec{
				ParentRefs: []v1.ParentReference{
					{
						Namespace:   (*v1.Namespace)(helpers.GetPointer("test")),
						Name:        v1.ObjectName(gateway),
						SectionName: (*v1.SectionName)(helpers.GetPointer(tlsListenerName)),
					},
				},
			},
			Hostnames: []v1.Hostname{
				v1.Hostname(hostname),
			},
			Rules: []v1alpha2.TLSRouteRule{
				{
					BackendRefs: backendRefs,
				},
			},
		},
	}
}

func createHTTPListener() v1.Listener {
	return v1.Listener{
		Name:     v1.SectionName(httpListenerName),
		Hostname: nil,
		Port:     80,
		Protocol: v1.HTTPProtocolType,
	}
}

func createHTTPSListener(name string, tlsSecret *apiv1.Secret) v1.Listener {
	return v1.Listener{
		Name:     v1.SectionName(name),
		Hostname: nil,
		Port:     443,
		Protocol: v1.HTTPSProtocolType,
		TLS: &v1.GatewayTLSConfig{
			Mode: helpers.GetPointer(v1.TLSModeTerminate),
			CertificateRefs: []v1.SecretObjectReference{
				{
					Kind:      (*v1.Kind)(helpers.GetPointer("Secret")),
					Name:      v1.ObjectName(tlsSecret.Name),
					Namespace: (*v1.Namespace)(&tlsSecret.Namespace),
				},
			},
		},
	}
}

func createTLSListener(name string) v1.Listener {
	return v1.Listener{
		Name:     v1.SectionName(name),
		Hostname: nil,
		Port:     8443,
		Protocol: v1.TLSProtocolType,
		TLS: &v1.GatewayTLSConfig{
			Mode: helpers.GetPointer(v1.TLSModePassthrough),
		},
	}
}

func createGateway(name string, listeners ...v1.Listener) *v1.Gateway {
	return &v1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  "test",
			Name:       name,
			Generation: 1,
		},
		Spec: v1.GatewaySpec{
			GatewayClassName: gcName,
			Listeners:        listeners,
		},
	}
}

func createRouteWithMultipleRules(
	name, gateway, hostname string,
	rules []v1.HTTPRouteRule,
) *v1.HTTPRoute {
	hr := createHTTPRoute(name, gateway, hostname)
	hr.Spec.Rules = rules

	return hr
}

func createHTTPRule(path string, backendRefs ...v1.HTTPBackendRef) v1.HTTPRouteRule {
	return v1.HTTPRouteRule{
		Matches: []v1.HTTPRouteMatch{
			{
				Path: &v1.HTTPPathMatch{
					Type:  helpers.GetPointer(v1.PathMatchPathPrefix),
					Value: &path,
				},
			},
		},
		BackendRefs: backendRefs,
	}
}

func createHTTPBackendRef(
	kind *v1.Kind,
	name v1.ObjectName,
	namespace *v1.Namespace,
) v1.HTTPBackendRef {
	return v1.HTTPBackendRef{
		BackendRef: v1.BackendRef{
			BackendObjectReference: createBackendRefObj(kind, name, namespace),
		},
	}
}

func createTLSBackendRef(name, namespace string) v1.BackendRef {
	kindSvc := v1.Kind("Service")
	ns := v1.Namespace(namespace)
	return v1.BackendRef{
		BackendObjectReference: createBackendRefObj(&kindSvc, v1.ObjectName(name), &ns),
	}
}

func createBackendRefObj(
	kind *v1.Kind,
	name v1.ObjectName,
	namespace *v1.Namespace,
) v1.BackendObjectReference {
	return v1.BackendObjectReference{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
		Port:      helpers.GetPointer[v1.PortNumber](80),
	}
}

func createRouteBackendRefs(refs []v1.HTTPBackendRef) []graph.RouteBackendRef {
	rbrs := make([]graph.RouteBackendRef, 0, len(refs))
	for _, ref := range refs {
		rbr := graph.RouteBackendRef{
			BackendRef: ref.BackendRef,
		}
		rbrs = append(rbrs, rbr)
	}
	return rbrs
}

func createGRPCRouteBackendRefs(refs []v1.GRPCBackendRef) []graph.RouteBackendRef {
	rbrs := make([]graph.RouteBackendRef, 0, len(refs))
	for _, ref := range refs {
		rbr := graph.RouteBackendRef{
			BackendRef: ref.BackendRef,
		}
		rbrs = append(rbrs, rbr)
	}
	return rbrs
}

func createAlwaysValidValidators() validation.Validators {
	return validation.Validators{
		HTTPFieldsValidator: &validationfakes.FakeHTTPFieldsValidator{},
		GenericValidator:    &validationfakes.FakeGenericValidator{},
		PolicyValidator:     &validationfakes.FakePolicyValidator{},
	}
}

func createScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()

	utilruntime.Must(v1.Install(scheme))
	utilruntime.Must(v1beta1.Install(scheme))
	utilruntime.Must(v1alpha2.Install(scheme))
	utilruntime.Must(v1alpha3.Install(scheme))
	utilruntime.Must(apiv1.AddToScheme(scheme))
	utilruntime.Must(discoveryV1.AddToScheme(scheme))
	utilruntime.Must(apiext.AddToScheme(scheme))
	utilruntime.Must(ngfAPIv1alpha1.AddToScheme(scheme))
	utilruntime.Must(ngfAPIv1alpha2.AddToScheme(scheme))

	return scheme
}

func getListenerByName(gw *graph.Gateway, name string) *graph.Listener {
	for _, l := range gw.Listeners {
		if l.Name == name {
			return l
		}
	}

	return nil
}

var (
	cert = []byte(`-----BEGIN CERTIFICATE-----
MIIDLjCCAhYCCQDAOF9tLsaXWjANBgkqhkiG9w0BAQsFADBaMQswCQYDVQQGEwJV
UzELMAkGA1UECAwCQ0ExITAfBgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5IEx0
ZDEbMBkGA1UEAwwSY2FmZS5leGFtcGxlLmNvbSAgMB4XDTE4MDkxMjE2MTUzNVoX
DTIzMDkxMTE2MTUzNVowWDELMAkGA1UEBhMCVVMxCzAJBgNVBAgMAkNBMSEwHwYD
VQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQxGTAXBgNVBAMMEGNhZmUuZXhh
bXBsZS5jb20wggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCp6Kn7sy81
p0juJ/cyk+vCAmlsfjtFM2muZNK0KtecqG2fjWQb55xQ1YFA2XOSwHAYvSdwI2jZ
ruW8qXXCL2rb4CZCFxwpVECrcxdjm3teViRXVsYImmJHPPSyQgpiobs9x7DlLc6I
BA0ZjUOyl0PqG9SJexMV73WIIa5rDVSF2r4kSkbAj4Dcj7LXeFlVXH2I5XwXCptC
n67JCg42f+k8wgzcRVp8XZkZWZVjwq9RUKDXmFB2YyN1XEWdZ0ewRuKYUJlsm692
skOrKQj0vkoPn41EE/+TaVEpqLTRoUY3rzg7DkdzfdBizFO2dsPNFx2CW0jXkNLv
Ko25CZrOhXAHAgMBAAEwDQYJKoZIhvcNAQELBQADggEBAKHFCcyOjZvoHswUBMdL
RdHIb383pWFynZq/LuUovsVA58B0Cg7BEfy5vWVVrq5RIkv4lZ81N29x21d1JH6r
jSnQx+DXCO/TJEV5lSCUpIGzEUYaUPgRyjsM/NUdCJ8uHVhZJ+S6FA+CnOD9rn2i
ZBePCI5rHwEXwnnl8ywij3vvQ5zHIuyBglWr/Qyui9fjPpwWUvUm4nv5SMG9zCV7
PpuwvuatqjO1208BjfE/cZHIg8Hw9mvW9x9C+IQMIMDE7b/g6OcK7LGTLwlFxvA8
7WjEequnayIphMhKRXVf1N349eN98Ez38fOTHTPbdJjFA/PcC+Gyme+iGt5OQdFh
yRE=
-----END CERTIFICATE-----`)
	key = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAqeip+7MvNadI7if3MpPrwgJpbH47RTNprmTStCrXnKhtn41k
G+ecUNWBQNlzksBwGL0ncCNo2a7lvKl1wi9q2+AmQhccKVRAq3MXY5t7XlYkV1bG
CJpiRzz0skIKYqG7Pcew5S3OiAQNGY1DspdD6hvUiXsTFe91iCGuaw1Uhdq+JEpG
wI+A3I+y13hZVVx9iOV8FwqbQp+uyQoONn/pPMIM3EVafF2ZGVmVY8KvUVCg15hQ
dmMjdVxFnWdHsEbimFCZbJuvdrJDqykI9L5KD5+NRBP/k2lRKai00aFGN684Ow5H
c33QYsxTtnbDzRcdgltI15DS7yqNuQmazoVwBwIDAQABAoIBAQCPSdSYnQtSPyql
FfVFpTOsoOYRhf8sI+ibFxIOuRauWehhJxdm5RORpAzmCLyL5VhjtJme223gLrw2
N99EjUKb/VOmZuDsBc6oCF6QNR58dz8cnORTewcotsJR1pn1hhlnR5HqJJBJask1
ZEnUQfcXZrL94lo9JH3E+Uqjo1FFs8xxE8woPBqjZsV7pRUZgC3LhxnwLSExyFo4
cxb9SOG5OmAJozStFoQ2GJOes8rJ5qfdvytgg9xbLaQL/x0kpQ62BoFMBDdqOePW
KfP5zZ6/07/vpj48yA1Q32PzobubsBLd3Kcn32jfm1E7prtWl+JeOFiOznBQFJbN
4qPVRz5hAoGBANtWyxhNCSLu4P+XgKyckljJ6F5668fNj5CzgFRqJ09zn0TlsNro
FTLZcxDqnR3HPYM42JERh2J/qDFZynRQo3cg3oeivUdBVGY8+FI1W0qdub/L9+yu
edOZTQ5XmGGp6r6jexymcJim/OsB3ZnYOpOrlD7SPmBvzNLk4MF6gxbXAoGBAMZO
0p6HbBmcP0tjFXfcKE77ImLm0sAG4uHoUx0ePj/2qrnTnOBBNE4MvgDuTJzy+caU
k8RqmdHCbHzTe6fzYq/9it8sZ77KVN1qkbIcuc+RTxA9nNh1TjsRne74Z0j1FCLk
hHcqH0ri7PYSKHTE8FvFCxZYdbuB84CmZihvxbpRAoGAIbjqaMYPTYuklCda5S79
YSFJ1JzZe1Kja//tDw1zFcgVCKa31jAwciz0f/lSRq3HS1GGGmezhPVTiqLfeZqc
R0iKbhgbOcVVkJJ3K0yAyKwPTumxKHZ6zImZS0c0am+RY9YGq5T7YrzpzcfvpiOU
ffe3RyFT7cfCmfoOhDCtzukCgYB30oLC1RLFOrqn43vCS51zc5zoY44uBzspwwYN
TwvP/ExWMf3VJrDjBCH+T/6sysePbJEImlzM+IwytFpANfiIXEt/48Xf60Nx8gWM
uHyxZZx/NKtDw0V8vX1POnq2A5eiKa+8jRARYKJLYNdfDuwolxvG6bZhkPi/4EtT
3Y18sQKBgHtKbk+7lNJVeswXE5cUG6EDUsDe/2Ua7fXp7FcjqBEoap1LSw+6TXp0
ZgrmKE8ARzM47+EJHUviiq/nupE15g0kJW3syhpU9zZLO7ltB0KIkO9ZRcmUjo8Q
cpLlHMAqbLJ8WYGJCkhiWxyal6hYTyWY4cVkC0xtTl/hUE9IeNKo
-----END RSA PRIVATE KEY-----`)
)

var _ = Describe("ChangeProcessor", func() {
	// graph outputs are large, so allow gomega to print everything on test failure
	format.MaxLength = 0
	Describe("Normal cases of processing changes", func() {
		var (
			gc = &v1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:       gcName,
					Generation: 1,
				},
				Spec: v1.GatewayClassSpec{
					ControllerName: controllerName,
				},
			}
			processor state.ChangeProcessor
		)

		testUpsertTriggersChange := func(obj client.Object) {
			processor.CaptureUpsertChange(obj)
			Expect(processor.Process()).ToNot(BeNil())
		}

		testUpsertDoesNotTriggerChange := func(obj client.Object) {
			processor.CaptureUpsertChange(obj)
			Expect(processor.Process()).To(BeNil())
		}

		testDeleteTriggersChange := func(obj client.Object, nsname types.NamespacedName) {
			processor.CaptureDeleteChange(obj, nsname)
			Expect(processor.Process()).ToNot(BeNil())
		}

		testDeleteDoesNotTriggerChange := func(obj client.Object, nsname types.NamespacedName) {
			processor.CaptureDeleteChange(obj, nsname)
			Expect(processor.Process()).To(BeNil())
		}

		BeforeEach(OncePerOrdered, func() {
			processor = state.NewChangeProcessorImpl(state.ChangeProcessorConfig{
				GatewayCtlrName:  controllerName,
				GatewayClassName: gcName,
				Logger:           logr.Discard(),
				Validators:       createAlwaysValidValidators(),
				MustExtractGVK:   kinds.NewMustExtractGKV(createScheme()),
			})
		})

		Describe("Process gateway resources", Ordered, func() {
			var (
				gcUpdated                                                  *v1.GatewayClass
				diffNsTLSSecret, sameNsTLSSecret                           *apiv1.Secret
				diffNsTLSCert, sameNsTLSCert                               *graph.CertificateBundle
				hr1, hr1Updated, hr2                                       *v1.HTTPRoute
				gr1, gr1Updated, gr2                                       *v1.GRPCRoute
				tr1, tr1Updated, tr2                                       *v1alpha2.TLSRoute
				gw1, gw1Updated, gw2, gw2Updated                           *v1.Gateway
				secretRefGrant, hrServiceRefGrant                          *v1beta1.ReferenceGrant
				grServiceRefGrant, trServiceRefGrant                       *v1beta1.ReferenceGrant
				expGraph, expGraph2                                        *graph.Graph
				expRouteHR1, expRouteHR2                                   *graph.L7Route
				expRouteGR1, expRouteGR2                                   *graph.L7Route
				expRouteTR1, expRouteTR2                                   *graph.L4Route
				gatewayAPICRD, gatewayAPICRDUpdated                        *metav1.PartialObjectMetadata
				httpRouteKey1, httpRouteKey2, grpcRouteKey1, grpcRouteKey2 graph.RouteKey // gitleaks:allow not a secret
				trKey1, trKey2                                             graph.L4RouteKey
				refSvc, refGRPCSvc, refTLSSvc                              types.NamespacedName
			)

			processAndValidateGraph := func(expGraph *graph.Graph) {
				graphCfg := processor.Process()
				Expect(helpers.Diff(expGraph, graphCfg)).To(BeEmpty())
				Expect(helpers.Diff(expGraph, processor.GetLatestGraph())).To(BeEmpty())
			}

			BeforeAll(func() {
				gcUpdated = gc.DeepCopy()
				gcUpdated.Generation++

				refSvc = types.NamespacedName{Namespace: "service-ns", Name: "service"}
				refGRPCSvc = types.NamespacedName{Namespace: "grpc-service-ns", Name: "grpc-service"}
				refTLSSvc = types.NamespacedName{Namespace: "tls-service-ns", Name: "tls-service"}

				crossNsHTTPBackendRef := v1.HTTPBackendRef{
					BackendRef: v1.BackendRef{
						BackendObjectReference: v1.BackendObjectReference{
							Kind:      helpers.GetPointer[v1.Kind]("Service"),
							Name:      v1.ObjectName(refSvc.Name),
							Namespace: helpers.GetPointer(v1.Namespace(refSvc.Namespace)),
							Port:      helpers.GetPointer[v1.PortNumber](80),
						},
					},
				}

				grpcBackendRef := v1.GRPCBackendRef{
					BackendRef: v1.BackendRef{
						BackendObjectReference: v1.BackendObjectReference{
							Kind:      helpers.GetPointer[v1.Kind]("Service"),
							Name:      v1.ObjectName(refGRPCSvc.Name),
							Namespace: helpers.GetPointer(v1.Namespace(refGRPCSvc.Namespace)),
							Port:      helpers.GetPointer[v1.PortNumber](80),
						},
					},
				}

				hr1 = createHTTPRoute("hr-1", "gateway-1", "foo.example.com", crossNsHTTPBackendRef)
				httpRouteKey1 = graph.CreateRouteKey(hr1)
				hr1Updated = hr1.DeepCopy()
				hr1Updated.Generation++

				hr2 = createHTTPRoute("hr-2", "gateway-2", "bar.example.com", crossNsHTTPBackendRef)
				httpRouteKey2 = graph.CreateRouteKey(hr2)

				gr1 = createGRPCRoute("gr-1", "gateway-1", "foo.example.com", grpcBackendRef)
				grpcRouteKey1 = graph.CreateRouteKey(gr1)
				gr1Updated = gr1.DeepCopy()
				gr1Updated.Generation++

				gr2 = createGRPCRoute("gr-2", "gateway-2", "bar.example.com", grpcBackendRef)
				grpcRouteKey2 = graph.CreateRouteKey(gr2)

				tlsBackendRef := createTLSBackendRef(refTLSSvc.Name, refTLSSvc.Namespace)
				tr1 = createTLSRoute("tr-1", "gateway-1", "foo.tls.com", tlsBackendRef)
				trKey1 = graph.CreateRouteKeyL4(tr1)
				tr1Updated = tr1.DeepCopy()
				tr1Updated.Generation++

				tr2 = createTLSRoute("tr-2", "gateway-2", "bar.tls.com", tlsBackendRef)
				trKey2 = graph.CreateRouteKeyL4(tr2)

				secretRefGrant = &v1beta1.ReferenceGrant{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "cert-ns",
						Name:      "ref-grant",
					},
					Spec: v1beta1.ReferenceGrantSpec{
						From: []v1beta1.ReferenceGrantFrom{
							{
								Group:     v1.GroupName,
								Kind:      kinds.Gateway,
								Namespace: "test",
							},
						},
						To: []v1beta1.ReferenceGrantTo{
							{
								Kind: "Secret",
							},
						},
					},
				}

				hrServiceRefGrant = &v1beta1.ReferenceGrant{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "service-ns",
						Name:      "ref-grant",
					},
					Spec: v1beta1.ReferenceGrantSpec{
						From: []v1beta1.ReferenceGrantFrom{
							{
								Group:     v1.GroupName,
								Kind:      kinds.HTTPRoute,
								Namespace: "test",
							},
						},
						To: []v1beta1.ReferenceGrantTo{
							{
								Kind: "Service",
							},
						},
					},
				}

				grServiceRefGrant = &v1beta1.ReferenceGrant{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "grpc-service-ns",
						Name:      "ref-grant",
					},
					Spec: v1beta1.ReferenceGrantSpec{
						From: []v1beta1.ReferenceGrantFrom{
							{
								Group:     v1.GroupName,
								Kind:      kinds.GRPCRoute,
								Namespace: "test",
							},
						},
						To: []v1beta1.ReferenceGrantTo{
							{
								Kind: "Service",
							},
						},
					},
				}

				trServiceRefGrant = &v1beta1.ReferenceGrant{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "tls-service-ns",
						Name:      "ref-grant",
					},
					Spec: v1beta1.ReferenceGrantSpec{
						From: []v1beta1.ReferenceGrantFrom{
							{
								Group:     v1.GroupName,
								Kind:      kinds.TLSRoute,
								Namespace: "test",
							},
						},
						To: []v1beta1.ReferenceGrantTo{
							{
								Kind: "Service",
							},
						},
					},
				}

				sameNsTLSSecret = &apiv1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "tls-secret",
						Namespace: "test",
					},
					Type: apiv1.SecretTypeTLS,
					Data: map[string][]byte{
						apiv1.TLSCertKey:       cert,
						apiv1.TLSPrivateKeyKey: key,
					},
				}
				sameNsTLSCert = graph.NewCertificateBundle(
					types.NamespacedName{Namespace: sameNsTLSSecret.Namespace, Name: sameNsTLSSecret.Name},
					"Secret",
					&graph.Certificate{
						TLSCert:       cert,
						TLSPrivateKey: key,
					},
				)

				diffNsTLSSecret = &apiv1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind: "Secret",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "different-ns-tls-secret",
						Namespace: "cert-ns",
					},
					Type: apiv1.SecretTypeTLS,
					Data: map[string][]byte{
						apiv1.TLSCertKey:       cert,
						apiv1.TLSPrivateKeyKey: key,
					},
				}

				diffNsTLSCert = graph.NewCertificateBundle(
					types.NamespacedName{Namespace: diffNsTLSSecret.Namespace, Name: diffNsTLSSecret.Name},
					"Secret",
					&graph.Certificate{
						TLSCert:       cert,
						TLSPrivateKey: key,
					},
				)

				gw1 = createGateway(
					"gateway-1",
					createHTTPListener(),
					createHTTPSListener(httpsListenerName, diffNsTLSSecret), // cert in diff namespace than gw
					createTLSListener(tlsListenerName),
				)

				gw1Updated = gw1.DeepCopy()
				gw1Updated.Generation++

				gw2 = createGateway(
					"gateway-2",
					createHTTPListener(),
					createHTTPSListener(httpsListenerName, sameNsTLSSecret),
					createTLSListener(tlsListenerName),
				)

				gw2Updated = gw2.DeepCopy()
				gw2Updated.Generation++

				gatewayAPICRD = &metav1.PartialObjectMetadata{
					TypeMeta: metav1.TypeMeta{
						Kind:       "CustomResourceDefinition",
						APIVersion: "apiextensions.k8s.io/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "gatewayclasses.gateway.networking.k8s.io",
						Annotations: map[string]string{
							graph.BundleVersionAnnotation: graph.SupportedVersion,
						},
					},
				}

				gatewayAPICRDUpdated = gatewayAPICRD.DeepCopy()
				gatewayAPICRDUpdated.Annotations[graph.BundleVersionAnnotation] = "v1.99.0"
			})
			BeforeEach(func() {
				expRouteHR1 = &graph.L7Route{
					Source:    hr1,
					RouteType: graph.RouteTypeHTTP,
					ParentRefs: []graph.ParentRef{
						{
							Attachment: &graph.ParentRefAttachmentStatus{
								AcceptedHostnames: map[string][]string{
									graph.CreateGatewayListenerKey(
										client.ObjectKeyFromObject(gw1),
										httpListenerName,
									): {"foo.example.com"},
								},
								Attached:     true,
								ListenerPort: 80,
							},
							Gateway: &graph.ParentRefGateway{
								NamespacedName: client.ObjectKeyFromObject(gw1),
							},
							SectionName: hr1.Spec.ParentRefs[0].SectionName,
						},
						{
							Attachment: &graph.ParentRefAttachmentStatus{
								AcceptedHostnames: map[string][]string{
									graph.CreateGatewayListenerKey(
										client.ObjectKeyFromObject(gw1),
										httpsListenerName,
									): {"foo.example.com"},
								},
								Attached:     true,
								ListenerPort: 443,
							},
							Gateway: &graph.ParentRefGateway{
								NamespacedName: client.ObjectKeyFromObject(gw1),
							},
							Idx:         1,
							SectionName: hr1.Spec.ParentRefs[1].SectionName,
						},
					},
					Spec: graph.L7RouteSpec{
						Hostnames: hr1.Spec.Hostnames,
						Rules: []graph.RouteRule{
							{
								BackendRefs: []graph.BackendRef{
									{
										SvcNsName:          refSvc,
										Weight:             1,
										InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
									},
								},
								ValidMatches: true,
								Filters: graph.RouteRuleFilters{
									Filters: []graph.Filter{},
									Valid:   true,
								},
								Matches:          hr1.Spec.Rules[0].Matches,
								RouteBackendRefs: createRouteBackendRefs(hr1.Spec.Rules[0].BackendRefs),
							},
						},
					},
					Valid:      true,
					Attachable: true,
					Conditions: []conditions.Condition{
						conditions.NewRouteBackendRefRefBackendNotFound(
							"spec.rules[0].backendRefs[0].name: Not found: \"service\"",
						),
					},
				}

				expRouteHR2 = &graph.L7Route{
					Source:    hr2,
					RouteType: graph.RouteTypeHTTP,
					ParentRefs: []graph.ParentRef{
						{
							Attachment: &graph.ParentRefAttachmentStatus{
								AcceptedHostnames: map[string][]string{
									graph.CreateGatewayListenerKey(
										client.ObjectKeyFromObject(gw2),
										httpListenerName,
									): {"bar.example.com"},
								},
								Attached:     true,
								ListenerPort: 80,
							},
							Gateway: &graph.ParentRefGateway{
								NamespacedName: client.ObjectKeyFromObject(gw2),
							},
							SectionName: hr2.Spec.ParentRefs[0].SectionName,
						},
						{
							Attachment: &graph.ParentRefAttachmentStatus{
								AcceptedHostnames: map[string][]string{
									graph.CreateGatewayListenerKey(
										client.ObjectKeyFromObject(gw2),
										httpsListenerName,
									): {"bar.example.com"},
								},
								Attached:     true,
								ListenerPort: 443,
							},
							Gateway: &graph.ParentRefGateway{
								NamespacedName: client.ObjectKeyFromObject(gw2),
							},
							Idx:         1,
							SectionName: hr2.Spec.ParentRefs[1].SectionName,
						},
					},
					Spec: graph.L7RouteSpec{
						Hostnames: hr2.Spec.Hostnames,
						Rules: []graph.RouteRule{
							{
								BackendRefs: []graph.BackendRef{
									{
										SvcNsName:          refSvc,
										Weight:             1,
										InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
									},
								},
								ValidMatches: true,
								Filters: graph.RouteRuleFilters{
									Valid:   true,
									Filters: []graph.Filter{},
								},
								Matches:          hr2.Spec.Rules[0].Matches,
								RouteBackendRefs: createRouteBackendRefs(hr2.Spec.Rules[0].BackendRefs),
							},
						},
					},
					Valid:      true,
					Attachable: true,
					Conditions: []conditions.Condition{
						conditions.NewRouteBackendRefRefBackendNotFound(
							"spec.rules[0].backendRefs[0].name: Not found: \"service\"",
						),
					},
				}

				expRouteGR1 = &graph.L7Route{
					Source:    gr1,
					RouteType: graph.RouteTypeGRPC,
					ParentRefs: []graph.ParentRef{
						{
							Attachment: &graph.ParentRefAttachmentStatus{
								AcceptedHostnames: map[string][]string{
									graph.CreateGatewayListenerKey(
										client.ObjectKeyFromObject(gw1),
										httpListenerName,
									): {"foo.example.com"},
								},
								Attached:     true,
								ListenerPort: 80,
							},
							Gateway: &graph.ParentRefGateway{
								NamespacedName: client.ObjectKeyFromObject(gw1),
							},
							SectionName: gr1.Spec.ParentRefs[0].SectionName,
						},
						{
							Attachment: &graph.ParentRefAttachmentStatus{
								AcceptedHostnames: map[string][]string{
									graph.CreateGatewayListenerKey(
										client.ObjectKeyFromObject(gw1),
										httpsListenerName,
									): {"foo.example.com"},
								},
								Attached:     true,
								ListenerPort: 443,
							},
							Gateway: &graph.ParentRefGateway{
								NamespacedName: client.ObjectKeyFromObject(gw1),
							},
							Idx:         1,
							SectionName: gr1.Spec.ParentRefs[1].SectionName,
						},
					},
					Spec: graph.L7RouteSpec{
						Hostnames: gr1.Spec.Hostnames,
						Rules: []graph.RouteRule{
							{
								BackendRefs: []graph.BackendRef{
									{
										SvcNsName:          refGRPCSvc,
										Weight:             1,
										InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
									},
								},
								ValidMatches: true,
								Filters: graph.RouteRuleFilters{
									Filters: []graph.Filter{},
									Valid:   true,
								},
								Matches:          graph.ConvertGRPCMatches(gr1.Spec.Rules[0].Matches),
								RouteBackendRefs: createGRPCRouteBackendRefs(gr1.Spec.Rules[0].BackendRefs),
							},
						},
					},
					Valid:      true,
					Attachable: true,
					Conditions: []conditions.Condition{
						conditions.NewRouteBackendRefRefBackendNotFound(
							"spec.rules[0].backendRefs[0].name: Not found: \"grpc-service\"",
						),
					},
				}

				expRouteGR2 = &graph.L7Route{
					Source:    gr2,
					RouteType: graph.RouteTypeGRPC,
					ParentRefs: []graph.ParentRef{
						{
							Attachment: &graph.ParentRefAttachmentStatus{
								AcceptedHostnames: map[string][]string{
									graph.CreateGatewayListenerKey(
										client.ObjectKeyFromObject(gw2),
										httpListenerName,
									): {"bar.example.com"},
								},
								Attached:     true,
								ListenerPort: 80,
							},
							Gateway: &graph.ParentRefGateway{
								NamespacedName: client.ObjectKeyFromObject(gw2),
							},
							SectionName: gr2.Spec.ParentRefs[0].SectionName,
						},
						{
							Attachment: &graph.ParentRefAttachmentStatus{
								AcceptedHostnames: map[string][]string{
									graph.CreateGatewayListenerKey(
										client.ObjectKeyFromObject(gw2),
										httpsListenerName,
									): {"bar.example.com"},
								},
								Attached:     true,
								ListenerPort: 443,
							},
							Gateway: &graph.ParentRefGateway{
								NamespacedName: client.ObjectKeyFromObject(gw2),
							},
							Idx:         1,
							SectionName: gr2.Spec.ParentRefs[1].SectionName,
						},
					},
					Spec: graph.L7RouteSpec{
						Hostnames: gr2.Spec.Hostnames,
						Rules: []graph.RouteRule{
							{
								BackendRefs: []graph.BackendRef{
									{
										SvcNsName:          refGRPCSvc,
										Weight:             1,
										InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
									},
								},
								ValidMatches: true,
								Filters: graph.RouteRuleFilters{
									Valid:   true,
									Filters: []graph.Filter{},
								},
								Matches:          graph.ConvertGRPCMatches(gr2.Spec.Rules[0].Matches),
								RouteBackendRefs: createGRPCRouteBackendRefs(gr2.Spec.Rules[0].BackendRefs),
							},
						},
					},
					Valid:      true,
					Attachable: true,
					Conditions: []conditions.Condition{
						conditions.NewRouteBackendRefRefBackendNotFound(
							"spec.rules[0].backendRefs[0].name: Not found: \"grpc-service\"",
						),
					},
				}

				expRouteTR1 = &graph.L4Route{
					Source: tr1,
					ParentRefs: []graph.ParentRef{
						{
							Attachment: &graph.ParentRefAttachmentStatus{
								AcceptedHostnames: map[string][]string{
									graph.CreateGatewayListenerKey(
										client.ObjectKeyFromObject(gw1),
										tlsListenerName,
									): {"foo.tls.com"},
								},
								Attached: true,
							},
							Gateway: &graph.ParentRefGateway{
								NamespacedName: client.ObjectKeyFromObject(gw1),
							},
							SectionName: tr1.Spec.ParentRefs[0].SectionName,
						},
					},
					Spec: graph.L4RouteSpec{
						Hostnames: tr1.Spec.Hostnames,
						BackendRef: graph.BackendRef{
							SvcNsName:          refTLSSvc,
							Valid:              false,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
						},
					},
					Valid:      true,
					Attachable: true,
					Conditions: []conditions.Condition{
						conditions.NewRouteBackendRefRefBackendNotFound(
							"spec.rules[0].backendRefs[0].name: Not found: \"tls-service\"",
						),
					},
				}

				expRouteTR2 = &graph.L4Route{
					Source: tr2,
					ParentRefs: []graph.ParentRef{
						{
							Attachment: &graph.ParentRefAttachmentStatus{
								AcceptedHostnames: map[string][]string{
									graph.CreateGatewayListenerKey(
										client.ObjectKeyFromObject(gw2),
										tlsListenerName,
									): {"bar.tls.com"},
								},
								Attached: true,
							},
							Gateway: &graph.ParentRefGateway{
								NamespacedName: client.ObjectKeyFromObject(gw2),
							},
							SectionName: tr2.Spec.ParentRefs[0].SectionName,
						},
					},
					Spec: graph.L4RouteSpec{
						Hostnames: tr2.Spec.Hostnames,
						BackendRef: graph.BackendRef{
							SvcNsName:          refTLSSvc,
							Valid:              false,
							InvalidForGateways: map[types.NamespacedName]conditions.Condition{},
						},
					},
					Valid:      true,
					Attachable: true,
					Conditions: []conditions.Condition{
						conditions.NewRouteBackendRefRefBackendNotFound(
							"spec.rules[0].backendRefs[0].name: Not found: \"tls-service\"",
						),
					},
				}

				// This is the base case expected graph. Tests will manipulate this to add or remove elements
				// to fit the expected output of the input under test.
				expGraph = &graph.Graph{
					GatewayClass: &graph.GatewayClass{
						Source: gc,
						Valid:  true,
					},
					Gateways: map[types.NamespacedName]*graph.Gateway{
						{Namespace: "test", Name: "gateway-1"}: {
							Source: gw1,
							Listeners: []*graph.Listener{
								{
									Name:        httpListenerName,
									GatewayName: client.ObjectKeyFromObject(gw1),
									Source:      gw1.Spec.Listeners[0],
									Valid:       true,
									Attachable:  true,
									Routes:      map[graph.RouteKey]*graph.L7Route{httpRouteKey1: expRouteHR1, grpcRouteKey1: expRouteGR1},
									L4Routes:    map[graph.L4RouteKey]*graph.L4Route{},
									SupportedKinds: []v1.RouteGroupKind{
										{Kind: v1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
										{Kind: v1.Kind(kinds.GRPCRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
									},
								},
								{
									Name:           httpsListenerName,
									GatewayName:    client.ObjectKeyFromObject(gw1),
									Source:         gw1.Spec.Listeners[1],
									Valid:          true,
									Attachable:     true,
									Routes:         map[graph.RouteKey]*graph.L7Route{httpRouteKey1: expRouteHR1, grpcRouteKey1: expRouteGR1},
									L4Routes:       map[graph.L4RouteKey]*graph.L4Route{},
									ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(diffNsTLSSecret)),
									SupportedKinds: []v1.RouteGroupKind{
										{Kind: v1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
										{Kind: v1.Kind(kinds.GRPCRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
									},
								},
								{
									Name:        tlsListenerName,
									GatewayName: client.ObjectKeyFromObject(gw1),
									Source:      gw1.Spec.Listeners[2],
									Valid:       true,
									Attachable:  true,
									Routes:      map[graph.RouteKey]*graph.L7Route{},
									L4Routes:    map[graph.L4RouteKey]*graph.L4Route{trKey1: expRouteTR1},
									SupportedKinds: []v1.RouteGroupKind{
										{Kind: v1.Kind(kinds.TLSRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
									},
								},
							},
							Valid: true,
							DeploymentName: types.NamespacedName{
								Namespace: "test",
								Name:      "gateway-1-test-class",
							},
						},
					},
					L4Routes:          map[graph.L4RouteKey]*graph.L4Route{trKey1: expRouteTR1},
					Routes:            map[graph.RouteKey]*graph.L7Route{httpRouteKey1: expRouteHR1, grpcRouteKey1: expRouteGR1},
					ReferencedSecrets: map[types.NamespacedName]*graph.Secret{},
					ReferencedServices: map[types.NamespacedName]*graph.ReferencedService{
						refSvc: {
							GatewayNsNames: map[types.NamespacedName]struct{}{{Namespace: "test", Name: "gateway-1"}: {}},
						},
						refTLSSvc: {
							GatewayNsNames: map[types.NamespacedName]struct{}{{Namespace: "test", Name: "gateway-1"}: {}},
						},
						refGRPCSvc: {
							GatewayNsNames: map[types.NamespacedName]struct{}{{Namespace: "test", Name: "gateway-1"}: {}},
						},
					},
				}

				expGraph2 = &graph.Graph{
					GatewayClass: &graph.GatewayClass{
						Source: gc,
						Valid:  true,
					},
					Gateways: map[types.NamespacedName]*graph.Gateway{
						{Namespace: "test", Name: "gateway-1"}: {
							Source: gw1,
							Listeners: []*graph.Listener{
								{
									Name:        httpListenerName,
									GatewayName: client.ObjectKeyFromObject(gw1),
									Source:      gw1.Spec.Listeners[0],
									Valid:       true,
									Attachable:  true,
									Routes:      map[graph.RouteKey]*graph.L7Route{httpRouteKey1: expRouteHR1, grpcRouteKey1: expRouteGR1},
									L4Routes:    map[graph.L4RouteKey]*graph.L4Route{},
									SupportedKinds: []v1.RouteGroupKind{
										{Kind: v1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
										{Kind: v1.Kind(kinds.GRPCRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
									},
								},
								{
									Name:           httpsListenerName,
									GatewayName:    client.ObjectKeyFromObject(gw1),
									Source:         gw1.Spec.Listeners[1],
									Valid:          true,
									Attachable:     true,
									Routes:         map[graph.RouteKey]*graph.L7Route{httpRouteKey1: expRouteHR1, grpcRouteKey1: expRouteGR1},
									L4Routes:       map[graph.L4RouteKey]*graph.L4Route{},
									ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(diffNsTLSSecret)),
									SupportedKinds: []v1.RouteGroupKind{
										{Kind: v1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
										{Kind: v1.Kind(kinds.GRPCRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
									},
								},
								{
									Name:        tlsListenerName,
									GatewayName: client.ObjectKeyFromObject(gw1),
									Source:      gw1.Spec.Listeners[2],
									Valid:       true,
									Attachable:  true,
									Routes:      map[graph.RouteKey]*graph.L7Route{},
									L4Routes:    map[graph.L4RouteKey]*graph.L4Route{trKey1: expRouteTR1},
									SupportedKinds: []v1.RouteGroupKind{
										{Kind: v1.Kind(kinds.TLSRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
									},
								},
							},
							Valid: true,
							DeploymentName: types.NamespacedName{
								Namespace: "test",
								Name:      "gateway-1-test-class",
							},
						},
						{Namespace: "test", Name: "gateway-2"}: {
							Source: gw2,
							Listeners: []*graph.Listener{
								{
									Name:        httpListenerName,
									GatewayName: client.ObjectKeyFromObject(gw2),
									Source:      gw2.Spec.Listeners[0],
									Valid:       true,
									Attachable:  true,
									Routes:      map[graph.RouteKey]*graph.L7Route{},
									L4Routes:    map[graph.L4RouteKey]*graph.L4Route{},
									SupportedKinds: []v1.RouteGroupKind{
										{Kind: v1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
										{Kind: v1.Kind(kinds.GRPCRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
									},
								},
								{
									Name:           httpsListenerName,
									GatewayName:    client.ObjectKeyFromObject(gw2),
									Source:         gw2.Spec.Listeners[1],
									Valid:          true,
									Attachable:     true,
									Routes:         map[graph.RouteKey]*graph.L7Route{},
									L4Routes:       map[graph.L4RouteKey]*graph.L4Route{},
									ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(sameNsTLSSecret)),
									SupportedKinds: []v1.RouteGroupKind{
										{Kind: v1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
										{Kind: v1.Kind(kinds.GRPCRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
									},
								},
								{
									Name:        tlsListenerName,
									GatewayName: client.ObjectKeyFromObject(gw2),
									Source:      gw2.Spec.Listeners[2],
									Valid:       true,
									Attachable:  true,
									Routes:      map[graph.RouteKey]*graph.L7Route{},
									L4Routes:    map[graph.L4RouteKey]*graph.L4Route{},
									SupportedKinds: []v1.RouteGroupKind{
										{Kind: v1.Kind(kinds.TLSRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
									},
								},
							},
							Valid: true,
							DeploymentName: types.NamespacedName{
								Namespace: "test",
								Name:      "gateway-2-test-class",
							},
						},
					},
					L4Routes: map[graph.L4RouteKey]*graph.L4Route{trKey1: expRouteTR1},
					Routes:   map[graph.RouteKey]*graph.L7Route{httpRouteKey1: expRouteHR1, grpcRouteKey1: expRouteGR1},
					ReferencedSecrets: map[types.NamespacedName]*graph.Secret{
						client.ObjectKeyFromObject(sameNsTLSSecret): {
							Source:     sameNsTLSSecret,
							CertBundle: sameNsTLSCert,
						},
						client.ObjectKeyFromObject(diffNsTLSSecret): {
							Source:     diffNsTLSSecret,
							CertBundle: diffNsTLSCert,
						},
					},
					ReferencedServices: map[types.NamespacedName]*graph.ReferencedService{
						refSvc: {
							GatewayNsNames: map[types.NamespacedName]struct{}{{Namespace: "test", Name: "gateway-1"}: {}},
						},
						refTLSSvc: {
							GatewayNsNames: map[types.NamespacedName]struct{}{{Namespace: "test", Name: "gateway-1"}: {}},
						},
						refGRPCSvc: {
							GatewayNsNames: map[types.NamespacedName]struct{}{{Namespace: "test", Name: "gateway-1"}: {}},
						},
					},
				}
			})
			When("no upsert has occurred", func() {
				It("returns nil graph", func() {
					graphCfg := processor.Process()
					Expect(graphCfg).To(BeNil())
					Expect(processor.GetLatestGraph()).To(BeNil())
				})
			})
			When("GatewayClass doesn't exist", func() {
				When("Gateway API CRD is added", func() {
					It("returns empty graph", func() {
						processor.CaptureUpsertChange(gatewayAPICRD)

						processAndValidateGraph(&graph.Graph{})
					})
				})
				When("Gateways don't exist", func() {
					When("the first HTTPRoute is upserted", func() {
						It("returns empty graph", func() {
							processor.CaptureUpsertChange(hr1)

							processAndValidateGraph(&graph.Graph{})
						})
					})
					When("the first GRPCRoute is upserted", func() {
						It("returns empty graph", func() {
							processor.CaptureUpsertChange(gr1)

							processAndValidateGraph(&graph.Graph{})
						})
					})
					When("the first TLSRoute is upserted", func() {
						It("returns empty graph", func() {
							processor.CaptureUpsertChange(tr1)

							processAndValidateGraph(&graph.Graph{})
						})
					})
					When("the different namespace TLS Secret is upserted", func() {
						It("returns nil graph", func() {
							processor.CaptureUpsertChange(diffNsTLSSecret)

							graphCfg := processor.Process()
							Expect(graphCfg).To(BeNil())
							Expect(helpers.Diff(&graph.Graph{}, processor.GetLatestGraph())).To(BeEmpty())
						})
					})
					When("the first Gateway is upserted", func() {
						It("returns populated graph", func() {
							processor.CaptureUpsertChange(gw1)

							expGraph.GatewayClass = nil

							gw := expGraph.Gateways[types.NamespacedName{Namespace: "test", Name: "gateway-1"}]
							gw.Conditions = conditions.NewGatewayInvalid("GatewayClass doesn't exist")
							gw.Valid = false
							gw.Listeners = nil

							// no ref grant exists yet for the routes
							expGraph.Routes[httpRouteKey1].Conditions = []conditions.Condition{
								conditions.NewRouteBackendRefRefNotPermitted(
									"spec.rules[0].backendRefs[0].namespace: Forbidden: " +
										"Backend ref to Service service-ns/service not permitted by any ReferenceGrant",
								),
							}

							expGraph.Routes[grpcRouteKey1].Conditions = []conditions.Condition{
								conditions.NewRouteBackendRefRefNotPermitted(
									"spec.rules[0].backendRefs[0].namespace: Forbidden: Backend ref to Service " +
										"grpc-service-ns/grpc-service not permitted by any ReferenceGrant",
								),
							}

							expGraph.L4Routes[trKey1].Conditions = []conditions.Condition{
								conditions.NewRouteBackendRefRefNotPermitted(
									"spec.rules[0].backendRefs[0].namespace: Forbidden: Backend ref to Service " +
										"tls-service-ns/tls-service not permitted by any ReferenceGrant",
								),
							}

							// gateway class does not exist so routes cannot attach
							expGraph.Routes[httpRouteKey1].ParentRefs[0].Attachment = &graph.ParentRefAttachmentStatus{
								AcceptedHostnames: map[string][]string{},
								FailedConditions:  []conditions.Condition{conditions.NewRouteNoMatchingParent()},
							}
							expGraph.Routes[httpRouteKey1].ParentRefs[1].Attachment = &graph.ParentRefAttachmentStatus{
								AcceptedHostnames: map[string][]string{},
								FailedConditions:  []conditions.Condition{conditions.NewRouteNoMatchingParent()},
							}
							expGraph.Routes[grpcRouteKey1].ParentRefs[0].Attachment = &graph.ParentRefAttachmentStatus{
								AcceptedHostnames: map[string][]string{},
								FailedConditions:  []conditions.Condition{conditions.NewRouteNoMatchingParent()},
							}
							expGraph.Routes[grpcRouteKey1].ParentRefs[1].Attachment = &graph.ParentRefAttachmentStatus{
								AcceptedHostnames: map[string][]string{},
								FailedConditions:  []conditions.Condition{conditions.NewRouteNoMatchingParent()},
							}
							expGraph.L4Routes[trKey1].ParentRefs[0].Attachment = &graph.ParentRefAttachmentStatus{
								AcceptedHostnames: map[string][]string{},
								FailedConditions:  []conditions.Condition{conditions.NewRouteNoMatchingParent()},
							}

							expGraph.ReferencedSecrets = nil
							expGraph.ReferencedServices = nil

							expRouteHR1.Spec.Rules[0].BackendRefs[0].SvcNsName = types.NamespacedName{}
							expRouteGR1.Spec.Rules[0].BackendRefs[0].SvcNsName = types.NamespacedName{}
							expRouteTR1.Spec.BackendRef.SvcNsName = types.NamespacedName{}

							processAndValidateGraph(expGraph)
						})
					})
				})
			})
			When("the GatewayClass is upserted", func() {
				It("returns updated graph", func() {
					processor.CaptureUpsertChange(gc)

					// No ref grant exists yet for gw1
					// so the listener is not valid, but still attachable
					gw := expGraph.Gateways[types.NamespacedName{Namespace: "test", Name: "gateway-1"}]
					listener443 := getListenerByName(gw, httpsListenerName)
					listener443.Valid = false
					listener443.ResolvedSecret = nil
					listener443.Conditions = conditions.NewListenerRefNotPermitted(
						"Certificate ref to secret cert-ns/different-ns-tls-secret not permitted by any ReferenceGrant",
					)

					expAttachment80 := &graph.ParentRefAttachmentStatus{
						AcceptedHostnames: map[string][]string{
							graph.CreateGatewayListenerKey(
								client.ObjectKeyFromObject(gw1),
								httpListenerName,
							): {"foo.example.com"},
						},
						Attached:     true,
						ListenerPort: 80,
					}

					expAttachment443 := &graph.ParentRefAttachmentStatus{
						AcceptedHostnames: map[string][]string{
							graph.CreateGatewayListenerKey(
								client.ObjectKeyFromObject(gw1),
								httpsListenerName,
							): {"foo.example.com"},
						},
						Attached:     true,
						ListenerPort: 443,
					}

					listener80 := getListenerByName(gw, httpListenerName)
					listener80.Routes[httpRouteKey1].ParentRefs[0].Attachment = expAttachment80
					listener443.Routes[httpRouteKey1].ParentRefs[1].Attachment = expAttachment443
					listener80.Routes[grpcRouteKey1].ParentRefs[0].Attachment = expAttachment80
					listener443.Routes[grpcRouteKey1].ParentRefs[1].Attachment = expAttachment443

					// no ref grant exists yet for hr1
					expGraph.Routes[httpRouteKey1].Conditions = []conditions.Condition{
						conditions.NewRouteBackendRefRefNotPermitted(
							"spec.rules[0].backendRefs[0].namespace: Forbidden: Backend ref to Service " +
								"service-ns/service not permitted by any ReferenceGrant",
						),
						conditions.NewRouteInvalidListener(),
					}
					expGraph.Routes[httpRouteKey1].ParentRefs[0].Attachment = expAttachment80
					expGraph.Routes[httpRouteKey1].ParentRefs[1].Attachment = expAttachment443

					// no ref grant exists yet for gr1
					expGraph.Routes[grpcRouteKey1].Conditions = []conditions.Condition{
						conditions.NewRouteBackendRefRefNotPermitted(
							"spec.rules[0].backendRefs[0].namespace: Forbidden: Backend ref to Service " +
								"grpc-service-ns/grpc-service not permitted by any ReferenceGrant",
						),
						conditions.NewRouteInvalidListener(),
					}
					expGraph.Routes[grpcRouteKey1].ParentRefs[0].Attachment = expAttachment80
					expGraph.Routes[grpcRouteKey1].ParentRefs[1].Attachment = expAttachment443

					// no ref grant exists yet for tr1
					expGraph.L4Routes[trKey1].Conditions = []conditions.Condition{
						conditions.NewRouteBackendRefRefNotPermitted(
							"spec.rules[0].backendRefs[0].namespace: Forbidden: Backend ref to Service " +
								"tls-service-ns/tls-service not permitted by any ReferenceGrant",
						),
					}

					expGraph.ReferencedSecrets = nil
					expGraph.ReferencedServices = nil

					expRouteHR1.Spec.Rules[0].BackendRefs[0].SvcNsName = types.NamespacedName{}
					expRouteGR1.Spec.Rules[0].BackendRefs[0].SvcNsName = types.NamespacedName{}
					expRouteTR1.Spec.BackendRef.SvcNsName = types.NamespacedName{}

					processAndValidateGraph(expGraph)
				})
			})
			When("the ReferenceGrant allowing the Gateway to reference its Secret is upserted", func() {
				It("returns updated graph", func() {
					processor.CaptureUpsertChange(secretRefGrant)

					// no ref grant exists yet for hr1
					expGraph.Routes[httpRouteKey1].Conditions = []conditions.Condition{
						conditions.NewRouteBackendRefRefNotPermitted(
							"spec.rules[0].backendRefs[0].namespace: Forbidden: Backend ref to Service " +
								"service-ns/service not permitted by any ReferenceGrant",
						),
					}

					// no ref grant exists yet for gr1
					expGraph.Routes[grpcRouteKey1].Conditions = []conditions.Condition{
						conditions.NewRouteBackendRefRefNotPermitted(
							"spec.rules[0].backendRefs[0].namespace: Forbidden: Backend ref to Service " +
								"grpc-service-ns/grpc-service not permitted by any ReferenceGrant",
						),
					}

					// no ref grant exists yet for tr1
					expGraph.L4Routes[trKey1].Conditions = []conditions.Condition{
						conditions.NewRouteBackendRefRefNotPermitted(
							"spec.rules[0].backendRefs[0].namespace: Forbidden: Backend ref to Service " +
								"tls-service-ns/tls-service not permitted by any ReferenceGrant",
						),
					}

					expGraph.ReferencedSecrets[client.ObjectKeyFromObject(diffNsTLSSecret)] = &graph.Secret{
						Source: diffNsTLSSecret,
						CertBundle: graph.NewCertificateBundle(
							types.NamespacedName{Namespace: diffNsTLSSecret.Namespace, Name: diffNsTLSSecret.Name},
							"Secret",
							&graph.Certificate{
								TLSCert:       cert,
								TLSPrivateKey: key,
							},
						),
					}

					expGraph.ReferencedServices = nil
					expRouteHR1.Spec.Rules[0].BackendRefs[0].SvcNsName = types.NamespacedName{}
					expRouteGR1.Spec.Rules[0].BackendRefs[0].SvcNsName = types.NamespacedName{}
					expRouteTR1.Spec.BackendRef.SvcNsName = types.NamespacedName{}

					processAndValidateGraph(expGraph)
				})
			})
			When("the ReferenceGrant allowing the hr1 to reference the Service in different ns is upserted", func() {
				It("returns updated graph", func() {
					processor.CaptureUpsertChange(hrServiceRefGrant)

					// no ref grant exists yet for gr1
					expGraph.Routes[grpcRouteKey1].Conditions = []conditions.Condition{
						conditions.NewRouteBackendRefRefNotPermitted(
							"spec.rules[0].backendRefs[0].namespace: Forbidden: Backend ref to Service " +
								"grpc-service-ns/grpc-service not permitted by any ReferenceGrant",
						),
					}
					delete(expGraph.ReferencedServices, refGRPCSvc)
					expRouteGR1.Spec.Rules[0].BackendRefs[0].SvcNsName = types.NamespacedName{}

					// no ref grant exists yet for tr1
					expGraph.L4Routes[trKey1].Conditions = []conditions.Condition{
						conditions.NewRouteBackendRefRefNotPermitted(
							"spec.rules[0].backendRefs[0].namespace: Forbidden: Backend ref to Service " +
								"tls-service-ns/tls-service not permitted by any ReferenceGrant",
						),
					}
					delete(expGraph.ReferencedServices, refTLSSvc)
					expRouteTR1.Spec.BackendRef.SvcNsName = types.NamespacedName{}

					expGraph.ReferencedSecrets[client.ObjectKeyFromObject(diffNsTLSSecret)] = &graph.Secret{
						Source: diffNsTLSSecret,
						CertBundle: graph.NewCertificateBundle(
							types.NamespacedName{Namespace: diffNsTLSSecret.Namespace, Name: diffNsTLSSecret.Name},
							"Secret",
							&graph.Certificate{
								TLSCert:       cert,
								TLSPrivateKey: key,
							},
						),
					}

					processAndValidateGraph(expGraph)
				})
			})
			When("the ReferenceGrant allowing the gr1 to reference the Service in different ns is upserted", func() {
				It("returns updated graph", func() {
					processor.CaptureUpsertChange(grServiceRefGrant)

					// no ref grant exists yet for tr1
					expGraph.L4Routes[trKey1].Conditions = []conditions.Condition{
						conditions.NewRouteBackendRefRefNotPermitted(
							"spec.rules[0].backendRefs[0].namespace: Forbidden: Backend ref to Service " +
								"tls-service-ns/tls-service not permitted by any ReferenceGrant",
						),
					}
					delete(expGraph.ReferencedServices, types.NamespacedName{Namespace: "tls-service-ns", Name: "tls-service"})
					expRouteTR1.Spec.BackendRef.SvcNsName = types.NamespacedName{}

					expGraph.ReferencedSecrets[client.ObjectKeyFromObject(diffNsTLSSecret)] = &graph.Secret{
						Source: diffNsTLSSecret,
						CertBundle: graph.NewCertificateBundle(
							types.NamespacedName{Namespace: diffNsTLSSecret.Namespace, Name: diffNsTLSSecret.Name},
							"Secret",
							&graph.Certificate{
								TLSCert:       cert,
								TLSPrivateKey: key,
							},
						),
					}

					processAndValidateGraph(expGraph)
				})
			})
			When("the ReferenceGrant allowing the tr1 to reference the Service in different ns is upserted", func() {
				It("returns updated graph", func() {
					processor.CaptureUpsertChange(trServiceRefGrant)

					expGraph.ReferencedSecrets[client.ObjectKeyFromObject(diffNsTLSSecret)] = &graph.Secret{
						Source:     diffNsTLSSecret,
						CertBundle: diffNsTLSCert,
					}

					processAndValidateGraph(expGraph)
				})
			})
			When("the Gateway API CRD with bundle version annotation change is processed", func() {
				It("returns updated graph", func() {
					processor.CaptureUpsertChange(gatewayAPICRDUpdated)

					expGraph.ReferencedSecrets[client.ObjectKeyFromObject(diffNsTLSSecret)] = &graph.Secret{
						Source:     diffNsTLSSecret,
						CertBundle: diffNsTLSCert,
					}

					expGraph.GatewayClass.Conditions = conditions.NewGatewayClassSupportedVersionBestEffort(
						graph.SupportedVersion,
					)

					processAndValidateGraph(expGraph)
				})
			})
			When("the Gateway API CRD without bundle version annotation change is processed", func() {
				It("returns nil graph", func() {
					gatewayAPICRDSameVersion := gatewayAPICRDUpdated.DeepCopy()

					processor.CaptureUpsertChange(gatewayAPICRDSameVersion)

					expGraph.ReferencedSecrets[client.ObjectKeyFromObject(diffNsTLSSecret)] = &graph.Secret{
						Source:     diffNsTLSSecret,
						CertBundle: diffNsTLSCert,
					}

					expGraph.GatewayClass.Conditions = conditions.NewGatewayClassSupportedVersionBestEffort(
						graph.SupportedVersion,
					)

					graphCfg := processor.Process()
					Expect(graphCfg).To(BeNil())
					Expect(helpers.Diff(expGraph, processor.GetLatestGraph())).To(BeEmpty())
				})
			})
			When("the Gateway API CRD with bundle version annotation change is processed", func() {
				It("returns updated graph", func() {
					// change back to supported version
					processor.CaptureUpsertChange(gatewayAPICRD)

					expGraph.ReferencedSecrets[client.ObjectKeyFromObject(diffNsTLSSecret)] = &graph.Secret{
						Source:     diffNsTLSSecret,
						CertBundle: diffNsTLSCert,
					}

					processAndValidateGraph(expGraph)
				})
			})
			When("the first HTTPRoute update with a generation changed is processed", func() {
				It("returns populated graph", func() {
					processor.CaptureUpsertChange(hr1Updated)

					gw := expGraph.Gateways[types.NamespacedName{Namespace: "test", Name: "gateway-1"}]
					listener443 := getListenerByName(gw, httpsListenerName)
					listener443.Routes[httpRouteKey1].Source.SetGeneration(hr1Updated.Generation)

					listener80 := getListenerByName(gw, httpListenerName)
					listener80.Routes[httpRouteKey1].Source.SetGeneration(hr1Updated.Generation)
					expGraph.ReferencedSecrets[client.ObjectKeyFromObject(diffNsTLSSecret)] = &graph.Secret{
						Source:     diffNsTLSSecret,
						CertBundle: diffNsTLSCert,
					}

					processAndValidateGraph(expGraph)
				},
				)
			})
			When("the first GRPCRoute update with a generation changed is processed", func() {
				It("returns populated graph", func() {
					processor.CaptureUpsertChange(gr1Updated)

					gw := expGraph.Gateways[types.NamespacedName{Namespace: "test", Name: "gateway-1"}]
					listener443 := getListenerByName(gw, httpsListenerName)
					listener443.Routes[grpcRouteKey1].Source.SetGeneration(gr1Updated.Generation)

					listener80 := getListenerByName(gw, httpListenerName)
					listener80.Routes[grpcRouteKey1].Source.SetGeneration(gr1Updated.Generation)
					expGraph.ReferencedSecrets[client.ObjectKeyFromObject(diffNsTLSSecret)] = &graph.Secret{
						Source:     diffNsTLSSecret,
						CertBundle: diffNsTLSCert,
					}

					processAndValidateGraph(expGraph)
				})
			})
			When("the first TLSRoute update with a generation changed is processed", func() {
				It("returns populated graph", func() {
					processor.CaptureUpsertChange(tr1Updated)

					gw := expGraph.Gateways[types.NamespacedName{Namespace: "test", Name: "gateway-1"}]
					tlsListener := getListenerByName(gw, tlsListenerName)
					tlsListener.L4Routes[trKey1].Source.SetGeneration(tr1Updated.Generation)

					expGraph.ReferencedSecrets[client.ObjectKeyFromObject(diffNsTLSSecret)] = &graph.Secret{
						Source:     diffNsTLSSecret,
						CertBundle: diffNsTLSCert,
					}

					processAndValidateGraph(expGraph)
				})
			})
			When("the first Gateway update with a generation changed is processed", func() {
				It("returns populated graph", func() {
					processor.CaptureUpsertChange(gw1Updated)

					gw := expGraph.Gateways[types.NamespacedName{Namespace: "test", Name: "gateway-1"}]
					gw.Source.Generation = gw1Updated.Generation
					expGraph.ReferencedSecrets[client.ObjectKeyFromObject(diffNsTLSSecret)] = &graph.Secret{
						Source:     diffNsTLSSecret,
						CertBundle: diffNsTLSCert,
					}

					processAndValidateGraph(expGraph)
				})
			})
			When("the GatewayClass update with generation change is processed", func() {
				It("returns populated graph", func() {
					processor.CaptureUpsertChange(gcUpdated)

					expGraph.GatewayClass.Source.Generation = gcUpdated.Generation
					expGraph.ReferencedSecrets[client.ObjectKeyFromObject(diffNsTLSSecret)] = &graph.Secret{
						Source:     diffNsTLSSecret,
						CertBundle: diffNsTLSCert,
					}

					processAndValidateGraph(expGraph)
				})
			})
			When("the different namespace TLS secret is upserted again", func() {
				It("returns populated graph", func() {
					processor.CaptureUpsertChange(diffNsTLSSecret)

					expGraph.ReferencedSecrets[client.ObjectKeyFromObject(diffNsTLSSecret)] = &graph.Secret{
						Source:     diffNsTLSSecret,
						CertBundle: diffNsTLSCert,
					}

					processAndValidateGraph(expGraph)
				})
			})
			When("no changes are captured", func() {
				It("returns nil graph", func() {
					expGraph.ReferencedSecrets[client.ObjectKeyFromObject(diffNsTLSSecret)] = &graph.Secret{
						Source:     diffNsTLSSecret,
						CertBundle: diffNsTLSCert,
					}

					graphCfg := processor.Process()
					Expect(graphCfg).To(BeNil())
					Expect(helpers.Diff(expGraph, processor.GetLatestGraph())).To(BeEmpty())
				})
			})
			When("the same namespace TLS Secret is upserted", func() {
				It("returns nil graph", func() {
					processor.CaptureUpsertChange(sameNsTLSSecret)

					expGraph.ReferencedSecrets[client.ObjectKeyFromObject(diffNsTLSSecret)] = &graph.Secret{
						Source:     diffNsTLSSecret,
						CertBundle: diffNsTLSCert,
					}

					graphCfg := processor.Process()
					Expect(graphCfg).To(BeNil())
					Expect(helpers.Diff(expGraph, processor.GetLatestGraph())).To(BeEmpty())
				})
			})
			When("the second Gateway is upserted", func() {
				It("returns populated graph with second gateway", func() {
					processor.CaptureUpsertChange(gw2)

					processAndValidateGraph(expGraph2)
				})
			})
			When("the second HTTPRoute is upserted", func() {
				It("returns populated graph", func() {
					processor.CaptureUpsertChange(hr2)

					expGraph2.ReferencedSecrets[client.ObjectKeyFromObject(diffNsTLSSecret)] = &graph.Secret{
						Source:     diffNsTLSSecret,
						CertBundle: diffNsTLSCert,
					}

					gw2NSName := client.ObjectKeyFromObject(gw2)
					gw := expGraph2.Gateways[gw2NSName]

					listener80 := getListenerByName(gw, httpListenerName)
					listener80.Routes = map[graph.RouteKey]*graph.L7Route{
						httpRouteKey2: expRouteHR2,
					}

					listener443 := getListenerByName(gw, httpsListenerName)
					listener443.Routes = map[graph.RouteKey]*graph.L7Route{
						httpRouteKey2: expRouteHR2,
					}

					expGraph2.Routes = map[graph.RouteKey]*graph.L7Route{
						httpRouteKey2: expRouteHR2,
						httpRouteKey1: expRouteHR1,
						grpcRouteKey1: expRouteGR1,
					}

					expGraph2.ReferencedServices[refSvc].GatewayNsNames[gw2NSName] = struct{}{}

					processAndValidateGraph(expGraph2)
				})
			})
			When("the second GRPCRoute is upserted", func() {
				It("returns populated graph", func() {
					processor.CaptureUpsertChange(gr2)

					gw2NSName := client.ObjectKeyFromObject(gw2)
					gw := expGraph2.Gateways[gw2NSName]

					listener80 := getListenerByName(gw, httpListenerName)
					listener80.Routes = map[graph.RouteKey]*graph.L7Route{
						httpRouteKey2: expRouteHR2,
						grpcRouteKey2: expRouteGR2,
					}

					listener443 := getListenerByName(gw, httpsListenerName)
					listener443.Routes = map[graph.RouteKey]*graph.L7Route{
						httpRouteKey2: expRouteHR2,
						grpcRouteKey2: expRouteGR2,
					}

					expGraph2.Routes = map[graph.RouteKey]*graph.L7Route{
						httpRouteKey2: expRouteHR2,
						httpRouteKey1: expRouteHR1,
						grpcRouteKey1: expRouteGR1,
						grpcRouteKey2: expRouteGR2,
					}

					expGraph2.ReferencedServices[refSvc].GatewayNsNames[gw2NSName] = struct{}{}
					expGraph2.ReferencedServices[refGRPCSvc].GatewayNsNames[gw2NSName] = struct{}{}

					processAndValidateGraph(expGraph2)
				})
			})
			When("the second TLSRoute is upserted", func() {
				It("returns populated graph", func() {
					processor.CaptureUpsertChange(tr2)

					gw2NSName := client.ObjectKeyFromObject(gw2)
					gw := expGraph2.Gateways[gw2NSName]

					listener80 := getListenerByName(gw, httpListenerName)
					listener80.Routes = map[graph.RouteKey]*graph.L7Route{
						httpRouteKey2: expRouteHR2,
						grpcRouteKey2: expRouteGR2,
					}

					listener443 := getListenerByName(gw, httpsListenerName)
					listener443.Routes = map[graph.RouteKey]*graph.L7Route{
						httpRouteKey2: expRouteHR2,
						grpcRouteKey2: expRouteGR2,
					}

					tlsListener := getListenerByName(gw, tlsListenerName)
					tlsListener.L4Routes = map[graph.L4RouteKey]*graph.L4Route{
						trKey2: expRouteTR2,
					}

					expGraph2.Routes = map[graph.RouteKey]*graph.L7Route{
						httpRouteKey2: expRouteHR2,
						httpRouteKey1: expRouteHR1,
						grpcRouteKey1: expRouteGR1,
						grpcRouteKey2: expRouteGR2,
					}

					expGraph2.L4Routes = map[graph.L4RouteKey]*graph.L4Route{
						trKey1: expRouteTR1,
						trKey2: expRouteTR2,
					}

					expGraph2.ReferencedServices[refSvc].GatewayNsNames[gw2NSName] = struct{}{}
					expGraph2.ReferencedServices[refGRPCSvc].GatewayNsNames[gw2NSName] = struct{}{}
					expGraph2.ReferencedServices[refTLSSvc].GatewayNsNames[gw2NSName] = struct{}{}

					processAndValidateGraph(expGraph2)
				})
			})
			When("the first Gateway is deleted", func() {
				It("returns updated graph", func() {
					processor.CaptureDeleteChange(
						&v1.Gateway{},
						types.NamespacedName{Namespace: "test", Name: "gateway-1"},
					)

					// gateway 2 only remains;
					expGraph2.Gateways = map[types.NamespacedName]*graph.Gateway{
						{Namespace: "test", Name: "gateway-2"}: {
							Source: gw2,
							Listeners: []*graph.Listener{
								{
									Name:        httpListenerName,
									GatewayName: client.ObjectKeyFromObject(gw2),
									Source:      gw2.Spec.Listeners[0],
									Valid:       true,
									Attachable:  true,
									Routes:      map[graph.RouteKey]*graph.L7Route{},
									L4Routes:    map[graph.L4RouteKey]*graph.L4Route{},
									SupportedKinds: []v1.RouteGroupKind{
										{Kind: v1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
										{Kind: v1.Kind(kinds.GRPCRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
									},
								},
								{
									Name:           httpsListenerName,
									GatewayName:    client.ObjectKeyFromObject(gw2),
									Source:         gw2.Spec.Listeners[1],
									Valid:          true,
									Attachable:     true,
									Routes:         map[graph.RouteKey]*graph.L7Route{},
									L4Routes:       map[graph.L4RouteKey]*graph.L4Route{},
									ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(sameNsTLSSecret)),
									SupportedKinds: []v1.RouteGroupKind{
										{Kind: v1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
										{Kind: v1.Kind(kinds.GRPCRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
									},
								},
								{
									Name:        tlsListenerName,
									GatewayName: client.ObjectKeyFromObject(gw2),
									Source:      gw2.Spec.Listeners[2],
									Valid:       true,
									Attachable:  true,
									Routes:      map[graph.RouteKey]*graph.L7Route{},
									L4Routes:    map[graph.L4RouteKey]*graph.L4Route{},
									SupportedKinds: []v1.RouteGroupKind{
										{Kind: v1.Kind(kinds.TLSRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
									},
								},
							},
							Valid: true,
							DeploymentName: types.NamespacedName{
								Namespace: "test",
								Name:      "gateway-2-test-class",
							},
						},
					}

					gw := expGraph2.Gateways[types.NamespacedName{Namespace: "test", Name: "gateway-2"}]

					listener80 := getListenerByName(gw, httpListenerName)
					listener80.Routes = map[graph.RouteKey]*graph.L7Route{
						httpRouteKey2: expRouteHR2,
						grpcRouteKey2: expRouteGR2,
					}

					listener443 := getListenerByName(gw, httpsListenerName)
					listener443.Routes = map[graph.RouteKey]*graph.L7Route{
						httpRouteKey2: expRouteHR2,
						grpcRouteKey2: expRouteGR2,
					}

					tlsListener := getListenerByName(gw, tlsListenerName)
					tlsListener.L4Routes = map[graph.L4RouteKey]*graph.L4Route{
						trKey2: expRouteTR2,
					}

					expGraph2.Routes = map[graph.RouteKey]*graph.L7Route{
						httpRouteKey2: expRouteHR2,
						grpcRouteKey2: expRouteGR2,
					}

					expGraph2.L4Routes = map[graph.L4RouteKey]*graph.L4Route{
						trKey2: expRouteTR2,
					}

					expGraph2.ReferencedServices = map[types.NamespacedName]*graph.ReferencedService{
						refSvc: {
							GatewayNsNames: map[types.NamespacedName]struct{}{
								{Namespace: "test", Name: "gateway-2"}: {},
							},
						},
						refTLSSvc: {
							GatewayNsNames: map[types.NamespacedName]struct{}{
								{Namespace: "test", Name: "gateway-2"}: {},
							},
						},
						refGRPCSvc: {
							GatewayNsNames: map[types.NamespacedName]struct{}{
								{Namespace: "test", Name: "gateway-2"}: {},
							},
						},
					}
					expGraph2.ReferencedSecrets = map[types.NamespacedName]*graph.Secret{
						client.ObjectKeyFromObject(sameNsTLSSecret): {
							Source:     sameNsTLSSecret,
							CertBundle: sameNsTLSCert,
						},
					}

					processAndValidateGraph(expGraph2)
				})
			})
			When("the second HTTPRoute is deleted", func() {
				It("returns updated graph", func() {
					processor.CaptureDeleteChange(
						&v1.HTTPRoute{},
						types.NamespacedName{Namespace: "test", Name: "hr-2"},
					)

					// gateway 2 only remains;
					expGraph2.Gateways = map[types.NamespacedName]*graph.Gateway{
						{Namespace: "test", Name: "gateway-2"}: {
							Source: gw2,
							Listeners: []*graph.Listener{
								{
									Name:        httpListenerName,
									GatewayName: client.ObjectKeyFromObject(gw2),
									Source:      gw2.Spec.Listeners[0],
									Valid:       true,
									Attachable:  true,
									Routes:      map[graph.RouteKey]*graph.L7Route{},
									L4Routes:    map[graph.L4RouteKey]*graph.L4Route{},
									SupportedKinds: []v1.RouteGroupKind{
										{Kind: v1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
										{Kind: v1.Kind(kinds.GRPCRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
									},
								},
								{
									Name:           httpsListenerName,
									GatewayName:    client.ObjectKeyFromObject(gw2),
									Source:         gw2.Spec.Listeners[1],
									Valid:          true,
									Attachable:     true,
									Routes:         map[graph.RouteKey]*graph.L7Route{},
									L4Routes:       map[graph.L4RouteKey]*graph.L4Route{},
									ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(sameNsTLSSecret)),
									SupportedKinds: []v1.RouteGroupKind{
										{Kind: v1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
										{Kind: v1.Kind(kinds.GRPCRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
									},
								},
								{
									Name:        tlsListenerName,
									GatewayName: client.ObjectKeyFromObject(gw2),
									Source:      gw2.Spec.Listeners[2],
									Valid:       true,
									Attachable:  true,
									Routes:      map[graph.RouteKey]*graph.L7Route{},
									L4Routes:    map[graph.L4RouteKey]*graph.L4Route{},
									SupportedKinds: []v1.RouteGroupKind{
										{Kind: v1.Kind(kinds.TLSRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
									},
								},
							},
							Valid: true,
							DeploymentName: types.NamespacedName{
								Namespace: "test",
								Name:      "gateway-2-test-class",
							},
						},
					}

					gw := expGraph2.Gateways[types.NamespacedName{Namespace: "test", Name: "gateway-2"}]

					listener80 := getListenerByName(gw, httpListenerName)
					listener80.Routes = map[graph.RouteKey]*graph.L7Route{
						grpcRouteKey2: expRouteGR2,
					}

					listener443 := getListenerByName(gw, httpsListenerName)
					listener443.Routes = map[graph.RouteKey]*graph.L7Route{
						grpcRouteKey2: expRouteGR2,
					}

					tlsListener := getListenerByName(gw, tlsListenerName)
					tlsListener.L4Routes = map[graph.L4RouteKey]*graph.L4Route{
						trKey2: expRouteTR2,
					}

					expGraph2.Routes = map[graph.RouteKey]*graph.L7Route{
						grpcRouteKey2: expRouteGR2,
					}

					expGraph2.L4Routes = map[graph.L4RouteKey]*graph.L4Route{
						trKey2: expRouteTR2,
					}

					expGraph2.ReferencedServices = map[types.NamespacedName]*graph.ReferencedService{
						refTLSSvc: {
							GatewayNsNames: map[types.NamespacedName]struct{}{
								{Namespace: "test", Name: "gateway-2"}: {},
							},
						},
						refGRPCSvc: {
							GatewayNsNames: map[types.NamespacedName]struct{}{
								{Namespace: "test", Name: "gateway-2"}: {},
							},
						},
					}
					expGraph2.ReferencedSecrets = map[types.NamespacedName]*graph.Secret{
						client.ObjectKeyFromObject(sameNsTLSSecret): {
							Source:     sameNsTLSSecret,
							CertBundle: sameNsTLSCert,
						},
					}
					processAndValidateGraph(expGraph2)
				})
			})
			When("the second GRPCRoute is deleted", func() {
				It("returns updated graph", func() {
					processor.CaptureDeleteChange(
						&v1.GRPCRoute{},
						types.NamespacedName{Namespace: "test", Name: "gr-2"},
					)

					// gateway 2 only remains;
					expGraph2.Gateways = map[types.NamespacedName]*graph.Gateway{
						{Namespace: "test", Name: "gateway-2"}: {
							Source: gw2,
							Listeners: []*graph.Listener{
								{
									Name:        httpListenerName,
									GatewayName: client.ObjectKeyFromObject(gw2),
									Source:      gw2.Spec.Listeners[0],
									Valid:       true,
									Attachable:  true,
									Routes:      map[graph.RouteKey]*graph.L7Route{},
									L4Routes:    map[graph.L4RouteKey]*graph.L4Route{},
									SupportedKinds: []v1.RouteGroupKind{
										{Kind: v1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
										{Kind: v1.Kind(kinds.GRPCRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
									},
								},
								{
									Name:           httpsListenerName,
									GatewayName:    client.ObjectKeyFromObject(gw2),
									Source:         gw2.Spec.Listeners[1],
									Valid:          true,
									Attachable:     true,
									Routes:         map[graph.RouteKey]*graph.L7Route{},
									L4Routes:       map[graph.L4RouteKey]*graph.L4Route{},
									ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(sameNsTLSSecret)),
									SupportedKinds: []v1.RouteGroupKind{
										{Kind: v1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
										{Kind: v1.Kind(kinds.GRPCRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
									},
								},
								{
									Name:        tlsListenerName,
									GatewayName: client.ObjectKeyFromObject(gw2),
									Source:      gw2.Spec.Listeners[2],
									Valid:       true,
									Attachable:  true,
									Routes:      map[graph.RouteKey]*graph.L7Route{},
									L4Routes:    map[graph.L4RouteKey]*graph.L4Route{},
									SupportedKinds: []v1.RouteGroupKind{
										{Kind: v1.Kind(kinds.TLSRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
									},
								},
							},
							Valid: true,
							DeploymentName: types.NamespacedName{
								Namespace: "test",
								Name:      "gateway-2-test-class",
							},
						},
					}

					gw := expGraph2.Gateways[types.NamespacedName{Namespace: "test", Name: "gateway-2"}]

					listener80 := getListenerByName(gw, httpListenerName)
					listener80.Routes = map[graph.RouteKey]*graph.L7Route{}

					listener443 := getListenerByName(gw, httpsListenerName)
					listener443.Routes = map[graph.RouteKey]*graph.L7Route{}

					tlsListener := getListenerByName(gw, tlsListenerName)
					tlsListener.L4Routes = map[graph.L4RouteKey]*graph.L4Route{
						trKey2: expRouteTR2,
					}

					expGraph2.Routes = map[graph.RouteKey]*graph.L7Route{}

					expGraph2.L4Routes = map[graph.L4RouteKey]*graph.L4Route{
						trKey2: expRouteTR2,
					}

					expGraph2.ReferencedServices = map[types.NamespacedName]*graph.ReferencedService{
						refTLSSvc: {
							GatewayNsNames: map[types.NamespacedName]struct{}{
								{Namespace: "test", Name: "gateway-2"}: {},
							},
						},
					}
					expGraph2.ReferencedSecrets = map[types.NamespacedName]*graph.Secret{
						client.ObjectKeyFromObject(sameNsTLSSecret): {
							Source:     sameNsTLSSecret,
							CertBundle: sameNsTLSCert,
						},
					}
					processAndValidateGraph(expGraph2)
				})
			})
			When("the second TLSRoute is deleted", func() {
				It("returns updated graph", func() {
					processor.CaptureDeleteChange(
						&v1alpha2.TLSRoute{},
						types.NamespacedName{Namespace: "test", Name: "tr-2"},
					)

					// gateway 2 only remains;
					expGraph2.Gateways = map[types.NamespacedName]*graph.Gateway{
						{Namespace: "test", Name: "gateway-2"}: {
							Source: gw2,
							Listeners: []*graph.Listener{
								{
									Name:        httpListenerName,
									GatewayName: client.ObjectKeyFromObject(gw2),
									Source:      gw2.Spec.Listeners[0],
									Valid:       true,
									Attachable:  true,
									Routes:      map[graph.RouteKey]*graph.L7Route{},
									L4Routes:    map[graph.L4RouteKey]*graph.L4Route{},
									SupportedKinds: []v1.RouteGroupKind{
										{Kind: v1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
										{Kind: v1.Kind(kinds.GRPCRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
									},
								},
								{
									Name:           httpsListenerName,
									GatewayName:    client.ObjectKeyFromObject(gw2),
									Source:         gw2.Spec.Listeners[1],
									Valid:          true,
									Attachable:     true,
									Routes:         map[graph.RouteKey]*graph.L7Route{},
									L4Routes:       map[graph.L4RouteKey]*graph.L4Route{},
									ResolvedSecret: helpers.GetPointer(client.ObjectKeyFromObject(sameNsTLSSecret)),
									SupportedKinds: []v1.RouteGroupKind{
										{Kind: v1.Kind(kinds.HTTPRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
										{Kind: v1.Kind(kinds.GRPCRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
									},
								},
								{
									Name:        tlsListenerName,
									GatewayName: client.ObjectKeyFromObject(gw2),
									Source:      gw2.Spec.Listeners[2],
									Valid:       true,
									Attachable:  true,
									Routes:      map[graph.RouteKey]*graph.L7Route{},
									L4Routes:    map[graph.L4RouteKey]*graph.L4Route{},
									SupportedKinds: []v1.RouteGroupKind{
										{Kind: v1.Kind(kinds.TLSRoute), Group: helpers.GetPointer[v1.Group](v1.GroupName)},
									},
								},
							},
							Valid: true,
							DeploymentName: types.NamespacedName{
								Namespace: "test",
								Name:      "gateway-2-test-class",
							},
						},
					}

					gw := expGraph2.Gateways[types.NamespacedName{Namespace: "test", Name: "gateway-2"}]

					listener80 := getListenerByName(gw, httpListenerName)
					listener80.Routes = map[graph.RouteKey]*graph.L7Route{}

					listener443 := getListenerByName(gw, httpsListenerName)
					listener443.Routes = map[graph.RouteKey]*graph.L7Route{}

					tlsListener := getListenerByName(gw, tlsListenerName)
					tlsListener.L4Routes = map[graph.L4RouteKey]*graph.L4Route{}

					expGraph2.Routes = map[graph.RouteKey]*graph.L7Route{}
					expGraph2.L4Routes = map[graph.L4RouteKey]*graph.L4Route{}

					expGraph2.ReferencedServices = nil
					expGraph2.ReferencedSecrets = map[types.NamespacedName]*graph.Secret{
						client.ObjectKeyFromObject(sameNsTLSSecret): {
							Source:     sameNsTLSSecret,
							CertBundle: sameNsTLSCert,
						},
					}
					processAndValidateGraph(expGraph2)
				})
			})
			When("the GatewayClass is deleted", func() {
				It("returns updated graph", func() {
					processor.CaptureDeleteChange(
						&v1.GatewayClass{},
						types.NamespacedName{Name: gcName},
					)

					expGraph2.GatewayClass = nil
					expGraph2.Gateways = map[types.NamespacedName]*graph.Gateway{
						{Namespace: "test", Name: "gateway-2"}: {
							Source: &v1.Gateway{
								ObjectMeta: metav1.ObjectMeta{
									Namespace:  "test",
									Name:       "gateway-2",
									Generation: 1,
								},
								Spec: v1.GatewaySpec{
									GatewayClassName: "test-class",
									Listeners: []v1.Listener{
										createHTTPListener(),
										createHTTPSListener(httpsListenerName, sameNsTLSSecret),
										createTLSListener(tlsListenerName),
									},
								},
							},
							Conditions: conditions.NewGatewayInvalid("GatewayClass doesn't exist"),
							DeploymentName: types.NamespacedName{
								Namespace: "test",
								Name:      "gateway-2-test-class",
							},
						},
					}
					expGraph2.Routes = map[graph.RouteKey]*graph.L7Route{}
					expGraph2.L4Routes = map[graph.L4RouteKey]*graph.L4Route{}
					expGraph2.ReferencedSecrets = nil

					expRouteHR1.Spec.Rules[0].BackendRefs[0].SvcNsName = types.NamespacedName{}
					expRouteGR1.Spec.Rules[0].BackendRefs[0].SvcNsName = types.NamespacedName{}
					expGraph2.ReferencedServices = nil

					processAndValidateGraph(expGraph2)
				})
			})
			When("the second Gateway is deleted", func() {
				It("returns empty graph", func() {
					processor.CaptureDeleteChange(
						&v1.Gateway{},
						types.NamespacedName{Namespace: "test", Name: "gateway-2"},
					)

					expRouteHR1.Spec.Rules[0].BackendRefs[0].SvcNsName = types.NamespacedName{}
					expRouteGR1.Spec.Rules[0].BackendRefs[0].SvcNsName = types.NamespacedName{}
					expGraph.ReferencedServices = nil

					processAndValidateGraph(&graph.Graph{})
				})
			})
			When("the first HTTPRoute is deleted", func() {
				It("returns empty graph", func() {
					processor.CaptureDeleteChange(
						&v1.HTTPRoute{},
						types.NamespacedName{Namespace: "test", Name: "hr-1"},
					)

					expRouteHR1.Spec.Rules[0].BackendRefs[0].SvcNsName = types.NamespacedName{}
					expGraph.ReferencedServices = nil

					processAndValidateGraph(&graph.Graph{})
				})
			})
			When("the first GRPCRoute is deleted", func() {
				It("returns empty graph", func() {
					processor.CaptureDeleteChange(
						&v1.GRPCRoute{},
						types.NamespacedName{Namespace: "test", Name: "gr-1"},
					)

					expRouteGR1.Spec.Rules[0].BackendRefs[0].SvcNsName = types.NamespacedName{}
					expGraph.ReferencedServices = nil

					processAndValidateGraph(&graph.Graph{})
				})
			})
			When("the first TLSRoute is deleted", func() {
				It("returns empty graph", func() {
					processor.CaptureDeleteChange(
						&v1alpha2.TLSRoute{},
						types.NamespacedName{Namespace: "test", Name: "tr-1"},
					)

					expGraph.ReferencedServices = nil

					processAndValidateGraph(&graph.Graph{})
				})
			})
		})

		Describe("Process services and endpoints", Ordered, func() {
			var (
				hr1, hr2, hr3, hrInvalidBackendRef, hrMultipleRules                 *v1.HTTPRoute
				hr1svc, sharedSvc, bazSvc1, bazSvc2, bazSvc3, invalidSvc, notRefSvc *apiv1.Service
				hr1slice1, hr1slice2, noRefSlice, missingSvcNameSlice               *discoveryV1.EndpointSlice
				gw                                                                  *v1.Gateway
				btls                                                                *v1alpha3.BackendTLSPolicy
			)

			createSvc := func(name string) *apiv1.Service {
				return &apiv1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      name,
					},
				}
			}

			createEndpointSlice := func(name string, svcName string) *discoveryV1.EndpointSlice {
				return &discoveryV1.EndpointSlice{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      name,
						Labels:    map[string]string{index.KubernetesServiceNameLabel: svcName},
					},
				}
			}

			createBackendTLSPolicy := func(name string, svcName string) *v1alpha3.BackendTLSPolicy {
				return &v1alpha3.BackendTLSPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      name,
					},
					Spec: v1alpha3.BackendTLSPolicySpec{
						TargetRefs: []v1alpha2.LocalPolicyTargetReferenceWithSectionName{
							{
								LocalPolicyTargetReference: v1alpha2.LocalPolicyTargetReference{
									Kind: v1.Kind("Service"),
									Name: v1.ObjectName(svcName),
								},
							},
						},
					},
				}
			}

			BeforeAll(func() {
				testNamespace := v1.Namespace("test")
				kindService := v1.Kind("Service")
				kindInvalid := v1.Kind("Invalid")

				// backend Refs
				fooRef := createHTTPBackendRef(&kindService, "foo-svc", &testNamespace)
				baz1NilNamespace := createHTTPBackendRef(&kindService, "baz-svc-v1", &testNamespace)
				barRef := createHTTPBackendRef(&kindService, "bar-svc", nil)
				baz2Ref := createHTTPBackendRef(&kindService, "baz-svc-v2", &testNamespace)
				baz3Ref := createHTTPBackendRef(&kindService, "baz-svc-v3", &testNamespace)
				invalidKindRef := createHTTPBackendRef(&kindInvalid, "bar-svc", &testNamespace)

				// httproutes
				hr1 = createHTTPRoute("hr1", "gw", "foo.example.com", fooRef)
				hr2 = createHTTPRoute("hr2", "gw", "bar.example.com", barRef)
				// hr3 shares the same backendRef as hr2
				hr3 = createHTTPRoute("hr3", "gw", "bar.2.example.com", barRef)
				hrInvalidBackendRef = createHTTPRoute("hr-invalid", "gw", "invalid.com", invalidKindRef)
				hrMultipleRules = createRouteWithMultipleRules(
					"hr-multiple-rules",
					"gw",
					"mutli.example.com",
					[]v1.HTTPRouteRule{
						createHTTPRule("/baz-v1", baz1NilNamespace),
						createHTTPRule("/baz-v2", baz2Ref),
						createHTTPRule("/baz-v3", baz3Ref),
					},
				)

				// services
				hr1svc = createSvc("foo-svc")
				sharedSvc = createSvc("bar-svc")  // shared between hr2 and hr3
				invalidSvc = createSvc("invalid") // nsname matches invalid BackendRef
				notRefSvc = createSvc("not-ref")
				bazSvc1 = createSvc("baz-svc-v1")
				bazSvc2 = createSvc("baz-svc-v2")
				bazSvc3 = createSvc("baz-svc-v3")

				// endpoint slices
				hr1slice1 = createEndpointSlice("hr1-1", "foo-svc")
				hr1slice2 = createEndpointSlice("hr1-2", "foo-svc")
				noRefSlice = createEndpointSlice("no-ref", "no-ref")
				missingSvcNameSlice = createEndpointSlice("missing-svc-name", "")

				// backendTLSPolicy
				btls = createBackendTLSPolicy("btls", "foo-svc")

				gw = createGateway("gw", createHTTPListener())
				processor.CaptureUpsertChange(gc)
				processor.CaptureUpsertChange(gw)
				gr := processor.Process()
				Expect(gr).ToNot(BeNil())
			})

			When("hr1 is added", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(hr1)
				})
			})
			When("a hr1 service is added", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(hr1svc)
				})
			})
			When("a backendTLSPolicy is added for referenced service", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(btls)
				})
			})
			When("an hr1 endpoint slice is added", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(hr1slice1)
				})
			})
			When("an hr1 service is updated", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(hr1svc)
				})
			})
			When("another hr1 endpoint slice is added", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(hr1slice2)
				})
			})
			When("an endpoint slice with a missing svc name label is added", func() {
				It("should not trigger a change", func() {
					testUpsertDoesNotTriggerChange(missingSvcNameSlice)
				})
			})
			When("an hr1 endpoint slice is deleted", func() {
				It("should trigger a change", func() {
					testDeleteTriggersChange(
						hr1slice1,
						types.NamespacedName{Namespace: hr1slice1.Namespace, Name: hr1slice1.Name},
					)
				})
			})
			When("the second hr1 endpoint slice is deleted", func() {
				It("should trigger a change", func() {
					testDeleteTriggersChange(
						hr1slice2,
						types.NamespacedName{Namespace: hr1slice2.Namespace, Name: hr1slice2.Name},
					)
				})
			})
			When("the second hr1 endpoint slice is recreated", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(hr1slice2)
				})
			})
			When("hr1 is deleted", func() {
				It("should trigger a change", func() {
					testDeleteTriggersChange(
						hr1,
						types.NamespacedName{Namespace: hr1.Namespace, Name: hr1.Name},
					)
				})
			})
			When("hr1 service is deleted", func() {
				It("should not trigger a change", func() {
					testDeleteDoesNotTriggerChange(
						hr1svc,
						types.NamespacedName{Namespace: hr1svc.Namespace, Name: hr1svc.Name},
					)
				})
			})
			When("the second hr1 endpoint slice is deleted", func() {
				It("should not trigger a change", func() {
					testDeleteDoesNotTriggerChange(
						hr1slice2,
						types.NamespacedName{Namespace: hr1slice2.Namespace, Name: hr1slice2.Name},
					)
				})
			})
			When("hr2 is added", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(hr2)
				})
			})
			When("a hr3, that shares a backend service with hr2, is added", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(hr3)
				})
			})
			When("sharedSvc, a service referenced by both hr2 and hr3, is added", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(sharedSvc)
				})
			})
			When("hr2 is deleted", func() {
				It("should trigger a change", func() {
					testDeleteTriggersChange(
						hr2,
						types.NamespacedName{Namespace: hr2.Namespace, Name: hr2.Name},
					)
				})
			})
			When("sharedSvc is deleted", func() {
				It("should trigger a change", func() {
					testDeleteTriggersChange(
						sharedSvc,
						types.NamespacedName{Namespace: sharedSvc.Namespace, Name: sharedSvc.Name},
					)
				})
			})
			When("sharedSvc is recreated", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(sharedSvc)
				})
			})
			When("hr3 is deleted", func() {
				It("should trigger a change", func() {
					testDeleteTriggersChange(
						hr3,
						types.NamespacedName{Namespace: hr3.Namespace, Name: hr3.Name},
					)
				})
			})
			When("sharedSvc is deleted", func() {
				It("should not trigger a change", func() {
					testDeleteDoesNotTriggerChange(
						sharedSvc,
						types.NamespacedName{Namespace: sharedSvc.Namespace, Name: sharedSvc.Name},
					)
				})
			})
			When("a service that is not referenced by any route is added", func() {
				It("should not trigger a change", func() {
					testUpsertDoesNotTriggerChange(notRefSvc)
				})
			})
			When("a route with an invalid backend ref type is added", func() {
				It("should trigger a change", func() {
					testUpsertTriggersChange(hrInvalidBackendRef)
				})
			})
			When("a service with a namespace name that matches invalid backend ref is added", func() {
				It("should not trigger a change", func() {
					testUpsertDoesNotTriggerChange(invalidSvc)
				})
			})
			When("an endpoint slice that is not owned by a referenced service is added", func() {
				It("should not trigger a change", func() {
					testUpsertDoesNotTriggerChange(noRefSlice)
				})
			})
			When("an endpoint slice that is not owned by a referenced service is deleted", func() {
				It("should not trigger a change", func() {
					testDeleteDoesNotTriggerChange(
						noRefSlice,
						types.NamespacedName{Namespace: noRefSlice.Namespace, Name: noRefSlice.Name},
					)
				})
			})
			Context("processing a route with multiple rules and three unique backend services", func() {
				When("route is added", func() {
					It("should trigger a change", func() {
						testUpsertTriggersChange(hrMultipleRules)
					})
				})
				When("first referenced service is added", func() {
					It("should trigger a change", func() {
						testUpsertTriggersChange(bazSvc1)
					})
				})
				When("second referenced service is added", func() {
					It("should trigger a change", func() {
						testUpsertTriggersChange(bazSvc2)
					})
				})
				When("first referenced service is deleted", func() {
					It("should trigger a change", func() {
						testDeleteTriggersChange(
							bazSvc1,
							types.NamespacedName{Namespace: bazSvc1.Namespace, Name: bazSvc1.Name},
						)
					})
				})
				When("first referenced service is recreated", func() {
					It("should trigger a change", func() {
						testUpsertTriggersChange(bazSvc1)
					})
				})
				When("third referenced service is added", func() {
					It("should trigger a change", func() {
						testUpsertTriggersChange(bazSvc3)
					})
				})
				When("third referenced service is updated", func() {
					It("should trigger a change", func() {
						testUpsertTriggersChange(bazSvc3)
					})
				})
				When("route is deleted", func() {
					It("should trigger a change", func() {
						testDeleteTriggersChange(
							hrMultipleRules,
							types.NamespacedName{
								Namespace: hrMultipleRules.Namespace,
								Name:      hrMultipleRules.Name,
							},
						)
					})
				})
				When("first referenced service is deleted", func() {
					It("should not trigger a change", func() {
						testDeleteDoesNotTriggerChange(
							bazSvc1,
							types.NamespacedName{Namespace: bazSvc1.Namespace, Name: bazSvc1.Name},
						)
					})
				})
				When("second referenced service is deleted", func() {
					It("should not trigger a change", func() {
						testDeleteDoesNotTriggerChange(
							bazSvc2,
							types.NamespacedName{Namespace: bazSvc2.Namespace, Name: bazSvc2.Name},
						)
					})
				})
				When("final referenced service is deleted", func() {
					It("should not trigger a change", func() {
						testDeleteDoesNotTriggerChange(
							bazSvc3,
							types.NamespacedName{Namespace: bazSvc3.Namespace, Name: bazSvc3.Name},
						)
					})
				})
			})
		})

		Describe("namespace changes", Ordered, func() {
			var (
				ns, nsDifferentLabels, nsNoLabels *apiv1.Namespace
				gw                                *v1.Gateway
			)

			BeforeAll(func() {
				ns = &apiv1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "ns",
						Labels: map[string]string{
							"app": "allowed",
						},
					},
				}
				nsDifferentLabels = &apiv1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "ns-different-labels",
						Labels: map[string]string{
							"oranges": "bananas",
						},
					},
				}
				nsNoLabels = &apiv1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "no-labels",
					},
				}
				gw = &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Name: "gw",
					},
					Spec: v1.GatewaySpec{
						GatewayClassName: gcName,
						Listeners: []v1.Listener{
							{
								Port:     80,
								Protocol: v1.HTTPProtocolType,
								AllowedRoutes: &v1.AllowedRoutes{
									Namespaces: &v1.RouteNamespaces{
										From: helpers.GetPointer(v1.NamespacesFromSelector),
										Selector: &metav1.LabelSelector{
											MatchLabels: map[string]string{
												"app": "allowed",
											},
										},
									},
								},
							},
						},
					},
				}
				processor = state.NewChangeProcessorImpl(state.ChangeProcessorConfig{
					GatewayCtlrName:  controllerName,
					GatewayClassName: gcName,
					Logger:           logr.Discard(),
					Validators:       createAlwaysValidValidators(),
					MustExtractGVK:   kinds.NewMustExtractGKV(createScheme()),
				})
				processor.CaptureUpsertChange(gc)
				processor.CaptureUpsertChange(gw)
				processor.Process()
			})

			When("a namespace is created that is not linked to a listener", func() {
				It("does not trigger an update", func() {
					testUpsertDoesNotTriggerChange(nsNoLabels)
				})
			})
			When("a namespace is created that is linked to a listener", func() {
				It("triggers an update", func() {
					testUpsertTriggersChange(ns)
				})
			})
			When("a namespace is deleted that is not linked to a listener", func() {
				It("does not trigger an update", func() {
					testDeleteDoesNotTriggerChange(nsNoLabels, types.NamespacedName{Name: "no-labels"})
				})
			})
			When("a namespace is deleted that is linked to a listener", func() {
				It("triggers an update", func() {
					testDeleteTriggersChange(ns, types.NamespacedName{Name: "ns"})
				})
			})
			When("a namespace that is not linked to a listener has its labels changed to match a listener", func() {
				It("triggers an update", func() {
					testUpsertDoesNotTriggerChange(nsDifferentLabels)
					nsDifferentLabels.Labels = map[string]string{
						"app": "allowed",
					}
					testUpsertTriggersChange(nsDifferentLabels)
				})
			})
			When(
				"a namespace that is linked to a listener has its labels changed to no longer match a listener",
				func() {
					It("triggers an update", func() {
						nsDifferentLabels.Labels = map[string]string{
							"oranges": "bananas",
						}
						testUpsertTriggersChange(nsDifferentLabels)
					})
				},
			)
			When("a gateway changes its listener's labels", func() {
				It("triggers an update when a namespace that matches the new labels is created", func() {
					gwChangedLabel := gw.DeepCopy()
					gwChangedLabel.Spec.Listeners[0].AllowedRoutes.Namespaces.Selector.MatchLabels = map[string]string{
						"oranges": "bananas",
					}
					gwChangedLabel.Generation++
					testUpsertTriggersChange(gwChangedLabel)

					// After changing the gateway's labels and generation, the processor should be marked to update
					// the nginx configuration and build a new graph. When processor.Process() gets called,
					// the nginx configuration gets updated and a new graph is built with an updated
					// referencedNamespaces. Thus, when the namespace "ns" is upserted with labels that no longer match
					// the new labels on the gateway, it would not trigger a change as the namespace would no longer
					// be in the updated referencedNamespaces and the labels no longer match the new labels on the
					// gateway.
					testUpsertDoesNotTriggerChange(ns)
					testUpsertTriggersChange(nsDifferentLabels)
				})
			})
			When("a namespace that is not linked to a listener has its labels removed", func() {
				It("does not trigger an update", func() {
					ns.Labels = nil
					testUpsertDoesNotTriggerChange(ns)
				})
			})
			When("a namespace that is linked to a listener has its labels removed", func() {
				It("triggers an update when labels are removed", func() {
					nsDifferentLabels.Labels = nil
					testUpsertTriggersChange(nsDifferentLabels)
				})
			})
		})

		Describe("NginxProxy resource changes", Ordered, func() {
			Context("referenced by a GatewayClass", func() {
				paramGC := gc.DeepCopy()
				paramGC.Spec.ParametersRef = &v1beta1.ParametersReference{
					Group:     ngfAPIv1alpha1.GroupName,
					Kind:      kinds.NginxProxy,
					Name:      "np",
					Namespace: helpers.GetPointer[v1.Namespace]("test"),
				}

				np := &ngfAPIv1alpha2.NginxProxy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "np",
						Namespace: "test",
					},
				}

				npUpdated := &ngfAPIv1alpha2.NginxProxy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "np",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha2.NginxProxySpec{
						Telemetry: &ngfAPIv1alpha2.Telemetry{
							Exporter: &ngfAPIv1alpha2.TelemetryExporter{
								Endpoint:   helpers.GetPointer("my-svc:123"),
								BatchSize:  helpers.GetPointer(int32(512)),
								BatchCount: helpers.GetPointer(int32(4)),
								Interval:   helpers.GetPointer(ngfAPIv1alpha1.Duration("5s")),
							},
						},
					},
				}
				It("handles upserts for an NginxProxy", func() {
					processor.CaptureUpsertChange(np)
					processor.CaptureUpsertChange(paramGC)

					graph := processor.Process()
					Expect(graph).ToNot(BeNil())
					Expect(graph.GatewayClass.NginxProxy.Source).To(Equal(np))
				})
				It("captures changes for an NginxProxy", func() {
					processor.CaptureUpsertChange(npUpdated)
					processor.CaptureUpsertChange(paramGC)

					graph := processor.Process()
					Expect(graph).ToNot(BeNil())
					Expect(graph.GatewayClass.NginxProxy.Source).To(Equal(npUpdated))
				})
				It("handles deletes for an NginxProxy", func() {
					processor.CaptureDeleteChange(np, client.ObjectKeyFromObject(np))

					graph := processor.Process()
					Expect(graph).ToNot(BeNil())
					Expect(graph.GatewayClass.NginxProxy).To(BeNil())
				})
			})
			Context("referenced by a Gateway", func() {
				paramGW := &v1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:  "test",
						Name:       "param-gw",
						Generation: 1,
					},
					Spec: v1.GatewaySpec{
						GatewayClassName: gcName,
						Listeners: []v1.Listener{
							{
								Name:     httpListenerName,
								Hostname: nil,
								Port:     80,
								Protocol: v1.HTTPProtocolType,
							},
						},
						Infrastructure: &v1.GatewayInfrastructure{
							ParametersRef: &v1.LocalParametersReference{
								Group: ngfAPIv1alpha1.GroupName,
								Kind:  kinds.NginxProxy,
								Name:  "np-gw",
							},
						},
					},
				}

				np := &ngfAPIv1alpha2.NginxProxy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "np-gw",
						Namespace: "test",
					},
				}

				npUpdated := &ngfAPIv1alpha2.NginxProxy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "np-gw",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha2.NginxProxySpec{
						Telemetry: &ngfAPIv1alpha2.Telemetry{
							Exporter: &ngfAPIv1alpha2.TelemetryExporter{
								Endpoint:   helpers.GetPointer("my-svc:123"),
								BatchSize:  helpers.GetPointer(int32(512)),
								BatchCount: helpers.GetPointer(int32(4)),
								Interval:   helpers.GetPointer(ngfAPIv1alpha1.Duration("5s")),
							},
						},
					},
				}
				It("handles upserts for an NginxProxy", func() {
					processor.CaptureUpsertChange(np)
					processor.CaptureUpsertChange(paramGW)

					graph := processor.Process()
					Expect(graph).ToNot(BeNil())
					gw := graph.Gateways[types.NamespacedName{Namespace: "test", Name: "param-gw"}]
					Expect(gw.NginxProxy.Source).To(Equal(np))
				})
				It("captures changes for an NginxProxy", func() {
					processor.CaptureUpsertChange(npUpdated)
					processor.CaptureUpsertChange(paramGW)

					graph := processor.Process()
					Expect(graph).ToNot(BeNil())
					gw := graph.Gateways[types.NamespacedName{Namespace: "test", Name: "param-gw"}]
					Expect(gw.NginxProxy.Source).To(Equal(npUpdated))
				})
				It("handles deletes for an NginxProxy", func() {
					processor.CaptureDeleteChange(np, client.ObjectKeyFromObject(np))

					graph := processor.Process()
					Expect(graph).ToNot(BeNil())
					gw := graph.Gateways[types.NamespacedName{Namespace: "test", Name: "param-gw"}]
					Expect(gw.NginxProxy).To(BeNil())
				})
			})
		})

		Describe("NGF Policy resource changes", Ordered, func() {
			var (
				gw                     *v1.Gateway
				route                  *v1.HTTPRoute
				svc                    *apiv1.Service
				csp, cspUpdated        *ngfAPIv1alpha1.ClientSettingsPolicy
				obs, obsUpdated        *ngfAPIv1alpha2.ObservabilityPolicy
				usp, uspUpdated        *ngfAPIv1alpha1.UpstreamSettingsPolicy
				cspKey, obsKey, uspKey graph.PolicyKey
			)

			BeforeAll(func() {
				processor.CaptureUpsertChange(gc)
				newGraph := processor.Process()
				Expect(newGraph).ToNot(BeNil())
				Expect(newGraph.GatewayClass.Source).To(Equal(gc))
				Expect(newGraph.NGFPolicies).To(BeEmpty())

				gw = createGateway("gw", createHTTPListener())
				route = createHTTPRoute(
					"hr-1",
					"gw",
					"foo.example.com",
					v1.HTTPBackendRef{
						BackendRef: v1.BackendRef{
							BackendObjectReference: v1.BackendObjectReference{
								Group: helpers.GetPointer[v1.Group](""),
								Kind:  helpers.GetPointer[v1.Kind](kinds.Service),
								Name:  "svc",
								Port:  helpers.GetPointer[v1.PortNumber](80),
							},
						},
					},
				)

				svc = &apiv1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "svc",
						Namespace: "test",
					},
				}

				csp = &ngfAPIv1alpha1.ClientSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "csp",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.ClientSettingsPolicySpec{
						TargetRef: v1alpha2.LocalPolicyTargetReference{
							Group: v1.GroupName,
							Kind:  kinds.Gateway,
							Name:  "gw",
						},
						Body: &ngfAPIv1alpha1.ClientBody{
							MaxSize: helpers.GetPointer[ngfAPIv1alpha1.Size]("10m"),
						},
					},
				}

				cspUpdated = csp.DeepCopy()
				cspUpdated.Spec.Body.MaxSize = helpers.GetPointer[ngfAPIv1alpha1.Size]("20m")

				cspKey = graph.PolicyKey{
					NsName: types.NamespacedName{Name: "csp", Namespace: "test"},
					GVK: schema.GroupVersionKind{
						Group:   ngfAPIv1alpha1.GroupName,
						Kind:    kinds.ClientSettingsPolicy,
						Version: "v1alpha1",
					},
				}

				obs = &ngfAPIv1alpha2.ObservabilityPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "obs",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha2.ObservabilityPolicySpec{
						TargetRefs: []v1alpha2.LocalPolicyTargetReference{
							{
								Group: v1.GroupName,
								Kind:  kinds.HTTPRoute,
								Name:  "hr-1",
							},
						},
						Tracing: &ngfAPIv1alpha2.Tracing{
							Strategy: ngfAPIv1alpha2.TraceStrategyRatio,
						},
					},
				}

				obsUpdated = obs.DeepCopy()
				obsUpdated.Spec.Tracing.Strategy = ngfAPIv1alpha2.TraceStrategyParent

				obsKey = graph.PolicyKey{
					NsName: types.NamespacedName{Name: "obs", Namespace: "test"},
					GVK: schema.GroupVersionKind{
						Group:   ngfAPIv1alpha1.GroupName,
						Kind:    kinds.ObservabilityPolicy,
						Version: "v1alpha2",
					},
				}

				usp = &ngfAPIv1alpha1.UpstreamSettingsPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "usp",
						Namespace: "test",
					},
					Spec: ngfAPIv1alpha1.UpstreamSettingsPolicySpec{
						ZoneSize: helpers.GetPointer[ngfAPIv1alpha1.Size]("10m"),
						TargetRefs: []v1alpha2.LocalPolicyTargetReference{
							{
								Group: "core",
								Kind:  kinds.Service,
								Name:  "svc",
							},
						},
					},
				}

				uspUpdated = usp.DeepCopy()
				uspUpdated.Spec.ZoneSize = helpers.GetPointer[ngfAPIv1alpha1.Size]("20m")

				uspKey = graph.PolicyKey{
					NsName: types.NamespacedName{Name: "usp", Namespace: "test"},
					GVK: schema.GroupVersionKind{
						Group:   ngfAPIv1alpha1.GroupName,
						Kind:    kinds.UpstreamSettingsPolicy,
						Version: "v1alpha1",
					},
				}
			})

			/*
				NOTE: When adding a new NGF policy to the change processor,
				update the following tests to make sure that the change processor can track changes for multiple NGF
				policies.
			*/

			When("a policy is created that references a resource that is not in the last graph", func() {
				It("reports no changes", func() {
					processor.CaptureUpsertChange(csp)
					processor.CaptureUpsertChange(obs)
					processor.CaptureUpsertChange(usp)

					Expect(processor.Process()).To(BeNil())
				})
			})
			When("the resource the policy references is created", func() {
				It("populates the graph with the policy", func() {
					processor.CaptureUpsertChange(gw)

					graph := processor.Process()
					Expect(graph).ToNot(BeNil())
					Expect(graph.NGFPolicies).To(HaveKey(cspKey))
					Expect(graph.NGFPolicies[cspKey].Source).To(Equal(csp))
					Expect(graph.NGFPolicies).ToNot(HaveKey(obsKey))

					processor.CaptureUpsertChange(route)
					graph = processor.Process()
					Expect(graph).ToNot(BeNil())
					Expect(graph.NGFPolicies).To(HaveKey(obsKey))
					Expect(graph.NGFPolicies[obsKey].Source).To(Equal(obs))

					processor.CaptureUpsertChange(svc)
					graph = processor.Process()
					Expect(graph).ToNot(BeNil())
					Expect(graph.NGFPolicies).To(HaveKey(uspKey))
					Expect(graph.NGFPolicies[uspKey].Source).To(Equal(usp))
				})
			})
			When("the policy is updated", func() {
				It("captures changes for a policy", func() {
					processor.CaptureUpsertChange(cspUpdated)
					processor.CaptureUpsertChange(obsUpdated)
					processor.CaptureUpsertChange(uspUpdated)

					graph := processor.Process()
					Expect(graph).ToNot(BeNil())
					Expect(graph.NGFPolicies).To(HaveKey(cspKey))
					Expect(graph.NGFPolicies[cspKey].Source).To(Equal(cspUpdated))
					Expect(graph.NGFPolicies).To(HaveKey(obsKey))
					Expect(graph.NGFPolicies[obsKey].Source).To(Equal(obsUpdated))
					Expect(graph.NGFPolicies).To(HaveKey(uspKey))
					Expect(graph.NGFPolicies[uspKey].Source).To(Equal(uspUpdated))
				})
			})
			When("the policy is deleted", func() {
				It("removes the policy from the graph", func() {
					processor.CaptureDeleteChange(&ngfAPIv1alpha1.ClientSettingsPolicy{}, client.ObjectKeyFromObject(csp))
					processor.CaptureDeleteChange(&ngfAPIv1alpha2.ObservabilityPolicy{}, client.ObjectKeyFromObject(obs))
					processor.CaptureDeleteChange(&ngfAPIv1alpha1.UpstreamSettingsPolicy{}, client.ObjectKeyFromObject(usp))

					graph := processor.Process()
					Expect(graph).ToNot(BeNil())
					Expect(graph.NGFPolicies).To(BeEmpty())
				})
			})
		})

		Describe("SnippetsFilter resource changed", Ordered, func() {
			sfNsName := types.NamespacedName{
				Name:      "sf",
				Namespace: "test",
			}

			sf := &ngfAPIv1alpha1.SnippetsFilter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sfNsName.Name,
					Namespace: sfNsName.Namespace,
				},
				Spec: ngfAPIv1alpha1.SnippetsFilterSpec{
					Snippets: []ngfAPIv1alpha1.Snippet{
						{
							Context: ngfAPIv1alpha1.NginxContextMain,
							Value:   "main snippet",
						},
					},
				},
			}

			sfUpdated := &ngfAPIv1alpha1.SnippetsFilter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sfNsName.Name,
					Namespace: sfNsName.Namespace,
				},
				Spec: ngfAPIv1alpha1.SnippetsFilterSpec{
					Snippets: []ngfAPIv1alpha1.Snippet{
						{
							Context: ngfAPIv1alpha1.NginxContextMain,
							Value:   "main snippet",
						},
						{
							Context: ngfAPIv1alpha1.NginxContextHTTP,
							Value:   "http snippet",
						},
					},
				},
			}
			It("handles upserts for a SnippetsFilter", func() {
				processor.CaptureUpsertChange(sf)

				graph := processor.Process()
				Expect(graph).ToNot(BeNil())

				processedSf, exists := graph.SnippetsFilters[sfNsName]
				Expect(exists).To(BeTrue())
				Expect(processedSf.Source).To(Equal(sf))
				Expect(processedSf.Valid).To(BeTrue())
			})
			It("captures changes for a SnippetsFilter", func() {
				processor.CaptureUpsertChange(sfUpdated)

				graph := processor.Process()
				Expect(graph).ToNot(BeNil())

				processedSf, exists := graph.SnippetsFilters[sfNsName]
				Expect(exists).To(BeTrue())
				Expect(processedSf.Source).To(Equal(sfUpdated))
				Expect(processedSf.Valid).To(BeTrue())
			})
			It("handles deletes for a SnippetsFilter", func() {
				processor.CaptureDeleteChange(sfUpdated, sfNsName)

				graph := processor.Process()
				Expect(graph).ToNot(BeNil())
				Expect(graph.SnippetsFilters).To(BeEmpty())
			})
		})
	})
	Describe("Ensuring non-changing changes don't override previously changing changes", func() {
		// Note: in these tests, we deliberately don't fully inspect the returned configuration and statuses
		// -- this is done in 'Normal cases of processing changes'

		var (
			processor                                                                         *state.ChangeProcessorImpl
			gcNsName, gwNsName, hrNsName, hr2NsName, grNsName, gr2NsName, rgNsName, svcNsName types.NamespacedName
			sliceNsName, secretNsName, cmNsName, btlsNsName, npNsName                         types.NamespacedName
			gc, gcUpdated                                                                     *v1.GatewayClass
			gw1, gw1Updated, gw2                                                              *v1.Gateway
			hr1, hr1Updated, hr2                                                              *v1.HTTPRoute
			gr1, gr1Updated, gr2                                                              *v1.GRPCRoute
			rg1, rg1Updated, rg2                                                              *v1beta1.ReferenceGrant
			svc, barSvc, unrelatedSvc                                                         *apiv1.Service
			slice, barSlice, unrelatedSlice                                                   *discoveryV1.EndpointSlice
			ns, unrelatedNS, testNs, barNs                                                    *apiv1.Namespace
			secret, secretUpdated, unrelatedSecret, barSecret, barSecretUpdated               *apiv1.Secret
			cm, cmUpdated, unrelatedCM                                                        *apiv1.ConfigMap
			btls, btlsUpdated                                                                 *v1alpha3.BackendTLSPolicy
			np, npUpdated                                                                     *ngfAPIv1alpha2.NginxProxy
		)

		BeforeEach(OncePerOrdered, func() {
			processor = state.NewChangeProcessorImpl(state.ChangeProcessorConfig{
				GatewayCtlrName:  "test.controller",
				GatewayClassName: "test-class",
				Validators:       createAlwaysValidValidators(),
				MustExtractGVK:   kinds.NewMustExtractGKV(createScheme()),
			})

			secretNsName = types.NamespacedName{Namespace: "test", Name: "tls-secret"}
			secret = &apiv1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:       secretNsName.Name,
					Namespace:  secretNsName.Namespace,
					Generation: 1,
				},
				Type: apiv1.SecretTypeTLS,
				Data: map[string][]byte{
					apiv1.TLSCertKey:       cert,
					apiv1.TLSPrivateKeyKey: key,
				},
			}
			secretUpdated = secret.DeepCopy()
			secretUpdated.Generation++
			barSecret = &apiv1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "bar-secret",
					Namespace:  "test",
					Generation: 1,
				},
				Type: apiv1.SecretTypeTLS,
				Data: map[string][]byte{
					apiv1.TLSCertKey:       cert,
					apiv1.TLSPrivateKeyKey: key,
				},
			}
			barSecretUpdated = barSecret.DeepCopy()
			barSecretUpdated.Generation++
			unrelatedSecret = &apiv1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "unrelated-tls-secret",
					Namespace:  "unrelated-ns",
					Generation: 1,
				},
				Type: apiv1.SecretTypeTLS,
				Data: map[string][]byte{
					apiv1.TLSCertKey:       cert,
					apiv1.TLSPrivateKeyKey: key,
				},
			}

			gcNsName = types.NamespacedName{Name: "test-class"}

			gc = &v1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: gcNsName.Name,
				},
				Spec: v1.GatewayClassSpec{
					ControllerName: "test.controller",
				},
			}

			gcUpdated = gc.DeepCopy()
			gcUpdated.Generation++

			gwNsName = types.NamespacedName{Namespace: "test", Name: "gw-1"}

			gw1 = &v1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "gw-1",
					Namespace:  "test",
					Generation: 1,
				},
				Spec: v1.GatewaySpec{
					GatewayClassName: gcName,
					Listeners: []v1.Listener{
						{
							Name:     httpListenerName,
							Hostname: nil,
							Port:     80,
							Protocol: v1.HTTPProtocolType,
							AllowedRoutes: &v1.AllowedRoutes{
								Namespaces: &v1.RouteNamespaces{
									From: helpers.GetPointer(v1.NamespacesFromSelector),
									Selector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"test": "namespace",
										},
									},
								},
							},
						},
						{
							Name:     httpsListenerName,
							Hostname: nil,
							Port:     443,
							Protocol: v1.HTTPSProtocolType,
							TLS: &v1.GatewayTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModeTerminate),
								CertificateRefs: []v1.SecretObjectReference{
									{
										Kind:      (*v1.Kind)(helpers.GetPointer("Secret")),
										Name:      v1.ObjectName(secret.Name),
										Namespace: (*v1.Namespace)(&secret.Namespace),
									},
								},
							},
						},
						{
							Name:     "listener-500-1",
							Hostname: nil,
							Port:     500,
							Protocol: v1.HTTPSProtocolType,
							TLS: &v1.GatewayTLSConfig{
								Mode: helpers.GetPointer(v1.TLSModeTerminate),
								CertificateRefs: []v1.SecretObjectReference{
									{
										Kind:      (*v1.Kind)(helpers.GetPointer("Secret")),
										Name:      v1.ObjectName(barSecret.Name),
										Namespace: (*v1.Namespace)(&barSecret.Namespace),
									},
								},
							},
						},
					},
				},
			}

			gw1Updated = gw1.DeepCopy()
			gw1Updated.Generation++

			gw2 = gw1.DeepCopy()
			gw2.Name = "gw-2"

			testNamespace := v1.Namespace("test")
			kindService := v1.Kind("Service")
			fooRef := createHTTPBackendRef(&kindService, "foo-svc", &testNamespace)
			barRef := createHTTPBackendRef(&kindService, "bar-svc", &testNamespace)

			hrNsName = types.NamespacedName{Namespace: "test", Name: "hr-1"}
			hr1 = createHTTPRoute("hr-1", "gw-1", "foo.example.com", fooRef, barRef)
			hr1Updated = hr1.DeepCopy()
			hr1Updated.Generation++
			hr2NsName = types.NamespacedName{Namespace: "test", Name: "hr-2"}
			hr2 = hr1.DeepCopy()
			hr2.Name = hr2NsName.Name

			grNsName = types.NamespacedName{Namespace: "test", Name: "gr-1"}
			gr1 = createGRPCRoute("gr-1", "gw-1", "foo.grpc.com")
			gr1Updated = gr1.DeepCopy()
			gr1Updated.Generation++
			gr2NsName = types.NamespacedName{Namespace: "test", Name: "hr-2"}
			gr2 = gr1.DeepCopy()
			gr2.Name = gr2NsName.Name

			svcNsName = types.NamespacedName{Namespace: "test", Name: "foo-svc"}
			svc = &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: svcNsName.Namespace,
					Name:      svcNsName.Name,
				},
			}
			barSvc = &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "bar-svc",
				},
			}
			unrelatedSvc = &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "unrelated-svc",
				},
			}

			sliceNsName = types.NamespacedName{Namespace: "test", Name: "slice"}
			slice = &discoveryV1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: sliceNsName.Namespace,
					Name:      sliceNsName.Name,
					Labels:    map[string]string{index.KubernetesServiceNameLabel: svc.Name},
				},
			}
			barSlice = &discoveryV1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "bar-slice",
					Labels:    map[string]string{index.KubernetesServiceNameLabel: "bar-svc"},
				},
			}
			unrelatedSlice = &discoveryV1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "unrelated-slice",
					Labels:    map[string]string{index.KubernetesServiceNameLabel: "unrelated-svc"},
				},
			}

			testNs = &apiv1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Labels: map[string]string{
						"test": "namespace",
					},
				},
			}
			ns = &apiv1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ns",
					Labels: map[string]string{
						"test": "namespace",
					},
				},
			}
			barNs = &apiv1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "bar-ns",
					Labels: map[string]string{
						"test": "namespace",
					},
				},
			}
			unrelatedNS = &apiv1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "unrelated-ns",
					Labels: map[string]string{
						"oranges": "bananas",
					},
				},
			}

			rgNsName = types.NamespacedName{Namespace: "test", Name: "rg-1"}

			rg1 = &v1beta1.ReferenceGrant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      rgNsName.Name,
					Namespace: rgNsName.Namespace,
				},
			}

			rg1Updated = rg1.DeepCopy()
			rg1Updated.Generation++

			rg2 = rg1.DeepCopy()
			rg2.Name = "rg-2"

			cmNsName = types.NamespacedName{Namespace: "test", Name: "cm-1"}
			cm = &apiv1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cmNsName.Name,
					Namespace: cmNsName.Namespace,
				},
				Data: map[string]string{
					"ca.crt": "value",
				},
			}
			cmUpdated = cm.DeepCopy()
			cmUpdated.Data["ca.crt"] = "updated-value"

			unrelatedCM = &apiv1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unrelated-cm",
					Namespace: "unrelated-ns",
				},
				Data: map[string]string{
					"ca.crt": "value",
				},
			}

			btlsNsName = types.NamespacedName{Namespace: "test", Name: "btls-1"}
			btls = &v1alpha3.BackendTLSPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:       btlsNsName.Name,
					Namespace:  btlsNsName.Namespace,
					Generation: 1,
				},
				Spec: v1alpha3.BackendTLSPolicySpec{
					TargetRefs: []v1alpha2.LocalPolicyTargetReferenceWithSectionName{
						{
							LocalPolicyTargetReference: v1alpha2.LocalPolicyTargetReference{
								Kind: "Service",
								Name: v1.ObjectName(svc.Name),
							},
						},
					},
					Validation: v1alpha3.BackendTLSPolicyValidation{
						CACertificateRefs: []v1.LocalObjectReference{
							{
								Name: v1.ObjectName(cm.Name),
							},
						},
					},
				},
			}
			btlsUpdated = btls.DeepCopy()

			npNsName = types.NamespacedName{Name: "np-1"}
			np = &ngfAPIv1alpha2.NginxProxy{
				ObjectMeta: metav1.ObjectMeta{
					Name: npNsName.Name,
				},
				Spec: ngfAPIv1alpha2.NginxProxySpec{
					Telemetry: &ngfAPIv1alpha2.Telemetry{
						ServiceName: helpers.GetPointer("my-svc"),
					},
				},
			}
			npUpdated = np.DeepCopy()
		})
		// Changing change - a change that makes processor.Process() return a built graph
		// Non-changing change - a change that doesn't do that
		// Related resource - a K8s resource that is related to a configured Gateway API resource
		// Unrelated resource - a K8s resource that is not related to a configured Gateway API resource

		// Note: in these tests, we deliberately don't fully inspect the returned configuration and statuses
		// -- this is done in 'Normal cases of processing changes'
		Describe("Multiple Gateway API resource changes", Ordered, func() {
			It("should build graph after multiple Upserts", func() {
				processor.CaptureUpsertChange(gc)
				processor.CaptureUpsertChange(gw1)
				processor.CaptureUpsertChange(testNs)
				processor.CaptureUpsertChange(hr1)
				processor.CaptureUpsertChange(gr1)
				processor.CaptureUpsertChange(rg1)
				processor.CaptureUpsertChange(btls)
				processor.CaptureUpsertChange(cm)
				processor.CaptureUpsertChange(np)

				Expect(processor.Process()).ToNot(BeNil())
			})
			When("a upsert of updated resources is followed by an upsert of the same generation", func() {
				It("should build graph", func() {
					// these are changing changes
					processor.CaptureUpsertChange(gcUpdated)
					processor.CaptureUpsertChange(gw1Updated)
					processor.CaptureUpsertChange(hr1Updated)
					processor.CaptureUpsertChange(gr1Updated)
					processor.CaptureUpsertChange(rg1Updated)
					processor.CaptureUpsertChange(btlsUpdated)
					processor.CaptureUpsertChange(cmUpdated)
					processor.CaptureUpsertChange(npUpdated)

					// there are non-changing changes
					processor.CaptureUpsertChange(gcUpdated)
					processor.CaptureUpsertChange(gw1Updated)
					processor.CaptureUpsertChange(hr1Updated)
					processor.CaptureUpsertChange(gr1Updated)
					processor.CaptureUpsertChange(rg1Updated)
					processor.CaptureUpsertChange(btlsUpdated)
					processor.CaptureUpsertChange(cmUpdated)
					processor.CaptureUpsertChange(npUpdated)

					Expect(processor.Process()).ToNot(BeNil())
				})
			})
			It("should build graph after upserting new resources", func() {
				// we can't have a second GatewayClass, so we don't add it
				processor.CaptureUpsertChange(gw2)
				processor.CaptureUpsertChange(hr2)
				processor.CaptureUpsertChange(gr2)
				processor.CaptureUpsertChange(rg2)

				Expect(processor.Process()).ToNot(BeNil())
			})
			When("resources are deleted followed by upserts with the same generations", func() {
				It("should build graph", func() {
					// these are changing changes
					processor.CaptureDeleteChange(&v1.GatewayClass{}, gcNsName)
					processor.CaptureDeleteChange(&v1.Gateway{}, gwNsName)
					processor.CaptureDeleteChange(&v1.HTTPRoute{}, hrNsName)
					processor.CaptureDeleteChange(&v1.GRPCRoute{}, grNsName)
					processor.CaptureDeleteChange(&v1beta1.ReferenceGrant{}, rgNsName)
					processor.CaptureDeleteChange(&v1alpha3.BackendTLSPolicy{}, btlsNsName)
					processor.CaptureDeleteChange(&apiv1.ConfigMap{}, cmNsName)
					processor.CaptureDeleteChange(&ngfAPIv1alpha2.NginxProxy{}, npNsName)

					// these are non-changing changes
					processor.CaptureUpsertChange(gw2)
					processor.CaptureUpsertChange(hr2)
					processor.CaptureUpsertChange(gr2)
					processor.CaptureUpsertChange(rg2)

					Expect(processor.Process()).ToNot(BeNil())
				})
			})
			It("should build graph after deleting resources", func() {
				processor.CaptureDeleteChange(&v1.HTTPRoute{}, hr2NsName)
				processor.CaptureDeleteChange(&v1.HTTPRoute{}, gr2NsName)

				Expect(processor.Process()).ToNot(BeNil())
			})
		})
		Describe("Deleting non-existing Gateway API resource", func() {
			It("should not build graph after deleting non-existing", func() {
				processor.CaptureDeleteChange(&v1.GatewayClass{}, gcNsName)
				processor.CaptureDeleteChange(&v1.Gateway{}, gwNsName)
				processor.CaptureDeleteChange(&v1.HTTPRoute{}, hrNsName)
				processor.CaptureDeleteChange(&v1.HTTPRoute{}, hr2NsName)
				processor.CaptureDeleteChange(&v1.HTTPRoute{}, grNsName)
				processor.CaptureDeleteChange(&v1.HTTPRoute{}, gr2NsName)
				processor.CaptureDeleteChange(&v1beta1.ReferenceGrant{}, rgNsName)

				Expect(processor.Process()).To(BeNil())
			})
		})
		Describe("Multiple Kubernetes API resource changes", Ordered, func() {
			BeforeAll(func() {
				// Set up graph
				processor.CaptureUpsertChange(gc)
				processor.CaptureUpsertChange(gw1)
				processor.CaptureUpsertChange(testNs)
				processor.CaptureUpsertChange(hr1)
				processor.CaptureUpsertChange(gr1)
				processor.CaptureUpsertChange(secret)
				processor.CaptureUpsertChange(barSecret)
				processor.CaptureUpsertChange(cm)
				Expect(processor.Process()).ToNot(BeNil())
			})

			It("should build graph after multiple Upserts of related resources", func() {
				processor.CaptureUpsertChange(svc)
				processor.CaptureUpsertChange(slice)
				processor.CaptureUpsertChange(ns)
				processor.CaptureUpsertChange(secretUpdated)
				processor.CaptureUpsertChange(cmUpdated)
				Expect(processor.Process()).ToNot(BeNil())
			})
			It("should not build graph after multiple Upserts of unrelated resources", func() {
				processor.CaptureUpsertChange(unrelatedSvc)
				processor.CaptureUpsertChange(unrelatedSlice)
				processor.CaptureUpsertChange(unrelatedNS)
				processor.CaptureUpsertChange(unrelatedSecret)
				processor.CaptureUpsertChange(unrelatedCM)

				Expect(processor.Process()).To(BeNil())
			})
			When("upserts of related resources are followed by upserts of unrelated resources", func() {
				It("should build graph", func() {
					// these are changing changes
					processor.CaptureUpsertChange(barSvc)
					processor.CaptureUpsertChange(barSlice)
					processor.CaptureUpsertChange(barNs)
					processor.CaptureUpsertChange(barSecretUpdated)
					processor.CaptureUpsertChange(cmUpdated)

					// there are non-changing changes
					processor.CaptureUpsertChange(unrelatedSvc)
					processor.CaptureUpsertChange(unrelatedSlice)
					processor.CaptureUpsertChange(unrelatedNS)
					processor.CaptureUpsertChange(unrelatedSecret)
					processor.CaptureUpsertChange(unrelatedCM)

					Expect(processor.Process()).ToNot(BeNil())
				})
			})
			When("deletes of related resources are followed by upserts of unrelated resources", func() {
				It("should build graph", func() {
					// these are changing changes
					processor.CaptureDeleteChange(&apiv1.Service{}, svcNsName)
					processor.CaptureDeleteChange(&discoveryV1.EndpointSlice{}, sliceNsName)
					processor.CaptureDeleteChange(&apiv1.Namespace{}, types.NamespacedName{Name: "ns"})
					processor.CaptureDeleteChange(&apiv1.Secret{}, secretNsName)
					processor.CaptureDeleteChange(&apiv1.ConfigMap{}, cmNsName)

					// these are non-changing changes
					processor.CaptureUpsertChange(unrelatedSvc)
					processor.CaptureUpsertChange(unrelatedSlice)
					processor.CaptureUpsertChange(unrelatedNS)
					processor.CaptureUpsertChange(unrelatedSecret)
					processor.CaptureUpsertChange(unrelatedCM)

					Expect(processor.Process()).ToNot(BeNil())
				})
			})
		})
		Describe("Multiple Kubernetes API and Gateway API resource changes", Ordered, func() {
			It("should build graph after multiple Upserts of new and related resources", func() {
				// new Gateway API resources
				processor.CaptureUpsertChange(gc)
				processor.CaptureUpsertChange(gw1)
				processor.CaptureUpsertChange(testNs)
				processor.CaptureUpsertChange(hr1)
				processor.CaptureUpsertChange(gr1)
				processor.CaptureUpsertChange(rg1)
				processor.CaptureUpsertChange(btls)

				// related Kubernetes API resources
				processor.CaptureUpsertChange(svc)
				processor.CaptureUpsertChange(slice)
				processor.CaptureUpsertChange(ns)
				processor.CaptureUpsertChange(secret)
				processor.CaptureUpsertChange(cm)

				Expect(processor.Process()).ToNot(BeNil())
			})
			It("should not build graph after multiple Upserts of unrelated resources", func() {
				// unrelated Kubernetes API resources
				processor.CaptureUpsertChange(unrelatedSvc)
				processor.CaptureUpsertChange(unrelatedSlice)
				processor.CaptureUpsertChange(unrelatedNS)
				processor.CaptureUpsertChange(unrelatedSecret)
				processor.CaptureUpsertChange(unrelatedCM)

				Expect(processor.Process()).To(BeNil())
			})
			It("should build graph after upserting changed resources followed by upserting unrelated resources",
				func() {
					// these are changing changes
					processor.CaptureUpsertChange(gcUpdated)
					processor.CaptureUpsertChange(gw1Updated)
					processor.CaptureUpsertChange(hr1Updated)
					processor.CaptureUpsertChange(gr1Updated)
					processor.CaptureUpsertChange(rg1Updated)
					processor.CaptureUpsertChange(btlsUpdated)

					// these are non-changing changes
					processor.CaptureUpsertChange(unrelatedSvc)
					processor.CaptureUpsertChange(unrelatedSlice)
					processor.CaptureUpsertChange(unrelatedNS)
					processor.CaptureUpsertChange(unrelatedSecret)
					processor.CaptureUpsertChange(unrelatedCM)

					Expect(processor.Process()).ToNot(BeNil())
				},
			)
		})
	})
	Describe("Edge cases with panic", func() {
		var processor state.ChangeProcessor

		BeforeEach(func() {
			processor = state.NewChangeProcessorImpl(state.ChangeProcessorConfig{
				GatewayCtlrName:  "test.controller",
				GatewayClassName: "my-class",
				Validators:       createAlwaysValidValidators(),
				MustExtractGVK:   kinds.NewMustExtractGKV(createScheme()),
			})
		})

		DescribeTable("CaptureUpsertChange must panic",
			func(obj client.Object) {
				process := func() {
					processor.CaptureUpsertChange(obj)
				}
				Expect(process).Should(Panic())
			},
			Entry(
				"an unsupported resource",
				&v1alpha2.TCPRoute{ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "tcp"}},
			),
			Entry(
				"nil resource",
				nil,
			),
		)

		DescribeTable(
			"CaptureDeleteChange must panic",
			func(resourceType ngftypes.ObjectType, nsname types.NamespacedName) {
				process := func() {
					processor.CaptureDeleteChange(resourceType, nsname)
				}
				Expect(process).Should(Panic())
			},
			Entry(
				"an unsupported resource",
				&v1alpha2.TCPRoute{},
				types.NamespacedName{Namespace: "test", Name: "tcp"},
			),
			Entry(
				"nil resource type",
				nil,
				types.NamespacedName{Namespace: "test", Name: "resource"},
			),
		)
	})
})
