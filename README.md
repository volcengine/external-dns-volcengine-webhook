# 概述
本项目 Volcengine Provider 是一个专为 ExternalDNS 设计的 Webhook 类型的提供商，其主要功能是将 ExternalDNS 与火山引擎（Volcengine）的 DNS 服务进行无缝集成。借助 Webhook 机制，ExternalDNS 能够把 DNS 记录的创建、更新以及删除操作传递给本服务，而本服务会负责将这些操作转化为针对火山引擎 DNS 服务的具体操作。

# 特性
- Webhook 集成：支持与 ExternalDNS 的 Webhook 集成，从而实现 DNS 记录的动态管理。
- 火山引擎 DNS 服务对接：可将 DNS 记录操作转发至火山引擎的 DNS 服务。
- 灵活配置：既支持通过配置文件进行详细配置，也支持利用环境变量实现灵活配置。

# 安装
前提条件
- 已安装 Helm 3.x
- 已配置火山引擎 API 密钥和 VPC 信息

## 使用 Helm 部署
1. 设置环境变量（或直接替换命令中的参数值）
```shell
   export VOLCENGINE_ACCESS_KEY="your-access-key"
   export VOLCENGINE_ACCESS_SECRET="your-access-secret"
   export VOLCENGINE_VPC="your-vpc-id"
   export VOLCENGINE_REGION="cn-beijing"  # 根据实际情况修改
   export TARGET_DOMAIN="test.com"
```
2. 执行 Helm 安装
```shell
   helm upgrade --install external-dns \
   manifests/externaldns \
   --namespace kube-system \
   --set userConfig.env.ak=${VOLCENGINE_ACCESS_KEY} \
   --set userConfig.env.sk=${VOLCENGINE_ACCESS_SECRET} \
   --set userConfig.env.vpc=${VOLCENGINE_VPC} \
   --set userConfig.env.region=${VOLCENGINE_REGION} \
   --set userConfig.env.endpoint=open.volcengineapi.com \
   --set userConfig.args.controller.domainFilter=${TARGET_DOMAIN} \  # 替换实际域名
   --set userConfig.args.provider.enableDebug=true \
   --set publicConfig.image.controller.repository=cr-helm-test-cn-beijing.cr.volces.com/test/external-dns \  # 替换实际镜像地址
   --set publicConfig.image.provider.repository=cr-helm-test-cn-beijing.cr.volces.com/test/external-dns-volcengine-webhook
```

3. 验证部署
```shell
   helm list -n kube-system
   kubectl get pods -n kube-system -l app.kubernetes.io/name=external-dns
```

## Helm 参数说明
|参数名称|描述|默认值|必填|
|-------|--|------|---|
|userConfig.env.ak|火山引擎 Access Key|无|是|
|userConfig.env.sk|火山引擎 Access Secret|无|是|
|userConfig.env.vpc|火山引擎 VPC ID|无|是|
|userConfig.env.region|火山引擎区域|cn-beijing|是|
|userConfig.env.endpoint|火山引擎 API 端点|open.volcengineapi.com|是|
|userConfig.args.controller.domainFilter|要管理的域名过滤器（支持逗号分隔多个域名）|cluster.local|是|
|userConfig.args.provider.enableDebug|是否启用调试模式|false|否|
|publicConfig.image.controller.repository|ExternalDNS 控制器镜像地址|暂时未定|否|
|publicConfig.image.provider.repository|Volcengine Webhook Provider 镜像地址|暂时未定|否|

提示：可通过 --set 参数覆盖默认值，或使用 -f values.yaml 文件进行批量配置。

values.yaml示例
```yaml
serviceAccount:
  create: true
  name: external-dns

publicConfig:
  deployMode: "Unmanaged"
  deployNodeType: "Node"

  image:
    controller:
      repository: cr-helm-test-cn-beijing.cr.volces.com/test/external-dns
      tag: v0.15.1
      pullPolicy: IfNotPresent
    provider:
      repository: cr-helm-test-cn-beijing.cr.volces.com/test/external-dns-provider
      tag: v0.15.1
      pullPolicy: Always



userConfig:
  Resources:
    controller:
      Requests:
        Cpu: "250m"
        Memory: "128Mi"
      Limits:
        Cpu: "500m"
        Memory: "256Mi"
    provider:
      Requests:
        Cpu: "250m"
        Memory: "128Mi"
      Limits:
        Cpu: "500m"
        Memory: "256Mi"
  args:
    controller:
      interval: 30s
      logLevel: info
      domainFilter:
    provider: 
      enableDebug: false


  env:
   vpc:
   region:
   endpoint:
   ak:
   sk: 
```