# Overview
Volcengine Provider is a webhook-based provider built for ExternalDNS.
It bridges ExternalDNS and Volcengine DNS so that every DNS record
(create, update, delete) emitted by ExternalDNS is translated into the
corresponding Volcengine DNS API call through a lightweight webhook
server.

# Features
- Webhook integration – dynamic DNS-record management via ExternalDNS
webhooks
- Volcengine DNS native – all operations are forwarded to Volcengine
DNS service
- Flexible configuration – configurable through files or environment
variables

# Installation
Prerequisites
- Helm 3.x installed
- [Optional] Volcengine API key (AK/SK) and VPC information ready
- [Optional] VKE IRSA ready

## [Optional] Prepare Volcengine API AK SK
Volcengine API key (AK/SK) should be created with the following permissions:
```json
{
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "private_zone:*"
      ],
      "Resource": [
        "*"
      ]
    }
  ]
}
```
## [Optional] Prepare VKE RISA
https://www.volcengine.com/docs/6460/1324604

## Deploy with Helm
1. Export environment variables
```shell
   export VOLCENGINE_CREDENTILALS_PROVIDER="aksk"         # support aksk or irsa
   export VOLCENGINE_SECRET_NAME="your-secret-with-ak-sk" # optional if use aksk
   export VOLCENGINE_OIDC_ROLE_TRN="your-oidc-role-trn"   # optional if use irsa
   export VOLCENGINE_AK="your-ak"
   export VOLCENGINE_SK="your-sk"
   export VOLCENGINE_VPC="your-vpc-id"
   export VOLCENGINE_REGION="cn-beijing"
   export VOLCENGINE_PRIVATEZONE_ENDPOINT="open.volcengineapi.com"
   export VOLCENGINE_STS_ENDPOINT="open.volcengineapi.com"
   export TARGET_DOMAINS="{test.com,test2.com}"
```

2. [Optional] Create the Secret
```shell
   kubectl create secret generic ${VOLCENGINE_SECRET_NAME} \
   --from-literal=access-key=${VOLCENGINE_AK} \
   --from-literal=secret-key=${VOLCENGINE_SK} \
   --namespace kube-system
```

3. Install the chart
```shell
   helm upgrade --install external-dns \
   manifests/externaldns \
   --namespace kube-system \
   --set userConfig.env.provider.credentialsProvider=${VOLCENGINE_CREDENTILALS_PROVIDER} \
   --set userConfig.env.provider.secretName=${VOLCENGINE_SECRET_NAME} \
   --set userConfig.env.provider.oidcRoleTrn=${VOLCENGINE_OIDC_ROLE_TRN} \
   --set userConfig.env.provider.vpc=${VOLCENGINE_VPC} \
   --set userConfig.env.provider.region=${VOLCENGINE_REGION} \
   --set userConfig.env.provider.privatezoneEndpoint=${VOLCENGINE_PRIVATEZONE_ENDPOINT} \
   --set userConfig.env.provider.stsEndpoint=${VOLCENGINE_STS_ENDPOINT} \
   --set userConfig.args.controller.domainFilter=${TARGET_DOMAINS} \
   --set publicConfig.image.controller.repository=registry.k8s.io/external-dns/external-dns \
   --set publicConfig.image.provider.repository=volcengine/external-dns-volcengine-webhook
```

4. Verify
```shell
   helm list -n kube-system
   kubectl get pods -n kube-system -l app.kubernetes.io/name=external-dns
```

## Helm parameters
| Parameter                                   | Description                                                                                                                                                               | Default                                    | Required |
|---------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------|----------|
| userConfig.env.provider.credentialsProvider | Provider used to obtain Volcengine API credentials. Valid values: aksk (default) or irsa                                                                                  | aksk                                       | yes      | 
| userConfig.env.provider.secretName          | Kubernetes secret that contains Volcengine Access Key (access-key) and Secret Key (secret-key), must set if `credentialsProvider=aksk`                                    | --                                         | no       |
| userConfig.env.provider.oidcRoleTrn         | Volcengine OpenID Connect (OIDC) role to assume for API access, must set if `credentialsProvider=irsa`                                                                    | --                                         | no       |
| userConfig.env.provider.vpc                 | Volcengine VPC identifier where the DNS zone is located.                                                                                                                  | --                                         | yes      |
| userConfig.env.provider.region              | Volcengine region in which the DNS zone resides.                                                                                                                          | cn-beijing                                 | yes      |
| userConfig.env.provider.privatezoneEndpoint | Custom Volcengine OpenAPI privatezone endpoint (overrides built-in global endpoint).                                                                                      | open.volcengineapi.com                     | yes      |
| userConfig.env.provider.stsEndpoint         | Custom Volcengine OpenAPI sts endpoint (overrides built-in global endpoint).                                                                                              | sts.volcengineapi.com                      | yes      |
| userConfig.args.controller.domainFilters    | Limit possible target zones by a list of domain suffixes; specify multiple times or use comma-separated values (same as --domain-filter).                                 | --                                         | yes      |
| userConfig.args.controller.policy           | How DNS records are synchronized between source and provider. Valid values: sync (create/update/delete) and upsert-only (create/update, never delete) (same as --policy). | upsert-only                                | no       |
| userConfig.args.controller.registry         | Registry implementation used to keep track of DNS record ownership. Valid values: txt (default TXT registry) or noop (no ownership records) (same as --registry).         | txt                                        | no       |
| userConfig.args.controller.txtOwnerId       | Identifier used as the owner for TXT registry records; must be unique across concurrent ExternalDNS instances (same as --txt-owner-id).                                   | --                                         | no       |
| userConfig.args.controller.txtPrefix        | Prefix added to ownership TXT record names to avoid collisions with real DNS records (same as --txt-prefix).                                                              | --                                         | no       |
| userConfig.args.provider.logLevel           | Enable verbose debug logging in the Volcengine webhook provider.                                                                                                          | info                                       | no       |
| publicConfig.image.controller.repository    | Container image for the ExternalDNS controller.                                                                                                                           | registry.k8s.io/external-dns/external-dns  | no       |
| publicConfig.image.provider.repository      | Container image for the Volcengine webhook provider.                                                                                                                      | volcengine/external-dns-volcengine-webhook | no       |

> Tip: override defaults with --set or supply a custom values.yaml via -f.

# Examples
external-dns support to reconcile LoadBalancer type service and ingress to dns record by default.

external-dns default reconcile policy is upsert-only.
## Service(LoadBalancer Type)
```shell
kubectl apply -f example/service.yaml
```
```yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx-service
  annotations:
    external-dns.alpha.kubernetes.io/hostname: nginx.test.com.  # expect to create record with hostname nginx.test.com
    external-dns.alpha.kubernetes.io/ttl: "300"                 # expect to set ttl to 300
spec:
  selector:
    app: nginx     
  ports:
    - protocol: TCP
      port: 80    
      targetPort: 80   
  type: LoadBalancer   # LoadBalancer
```

## Ingress
```shell
kubectl apply -f example/ingress.yaml
```
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    vke.volcengine.com/ingress-type: ingress-nginx
    external-dns.alpha.kubernetes.io/ttl: "300" # expect to set ttl to 300
  name: nginx-ingress-external
spec:
  ingressClassName: nginx
  rules:
    - host: nginx-ingress-external.test.com   # expect to create record with hostname nginx-ingress-external.test.com
      http:
        paths:
          - backend:
              service:
                name: nginx-service
                port:
                  number: 80
            path: /path1
            pathType: Prefix
    - host: nginx-ingress-external2.test.com  # expect to create record with hostname nginx-ingress-external2.test.com
      http:
        paths:
          - backend:
              service:
                name: nginx-service
                port:
                  number: 80
            path: /path2
            pathType: Prefix
```
