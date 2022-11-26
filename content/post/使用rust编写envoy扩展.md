---
title: "使用rust编写envoy扩展"
date: 2022-11-25T10:11:19+08:00
tags: ["envoy"]
---

## 最简单使用

先最简单的使用`envoy` 代理一个网站

下载envoy二进制

```bash
wget https://github.com/envoyproxy/envoy/releases/download/v1.24.0/envoy-1.24.0-linux-x86_64
```

设置一个配置文件,示例文件来自于 [官方文档](https://www.envoyproxy.io/docs/envoy/latest/start/quick-start/configuration-static)

```yaml
admin:
  address:
    socket_address: { address: 0.0.0.0, port_value: 9901 }

static_resources:

  listeners:
  - name: listener_0
    address:
      socket_address:
        address: 0.0.0.0
        port_value: 10000
    filter_chains:
    - filters:
      - name: envoy.filters.network.http_connection_manager
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          stat_prefix: ingress_http
          access_log:
          - name: envoy.access_loggers.stdout
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.access_loggers.stream.v3.StdoutAccessLog
          http_filters:
          - name: envoy.filters.http.router
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.router.v3.Router
          route_config:
            name: local_route
            virtual_hosts:
            - name: local_service
              domains: ["*"]
              routes:
              - match:
                  prefix: "/"
                route:
                  host_rewrite_literal: www.baidu.com
                  cluster: service_baidu_io

  clusters:
  - name: service_baidu_io
    type: LOGICAL_DNS
    # Comment out the following line to test on v6 networks
    dns_lookup_family: V4_ONLY
    load_assignment:
      cluster_name: service_baidu_io
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              socket_address:
                address: www.baidu.com
                port_value: 443
    transport_socket:
      name: envoy.transport_sockets.tls
      typed_config:
        "@type": type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext
        sni: www.baidu.com
```

启动访问 主机的`10000` 端口,发现请求被转发到了百度

```
./envoy -c envoy.yaml
```

## 使用rust编写简单扩展

写一个简单的示例

```rust
use log::info;
use proxy_wasm::traits::*;
use proxy_wasm::types::*;

proxy_wasm::main! {{
    proxy_wasm::set_log_level(proxy_wasm::types::LogLevel::Trace);
    proxy_wasm::set_root_context(|_| -> Box<dyn RootContext>
        { Box::new(TestWasm) });
}}


struct TestWasm;

impl Context for TestWasm {}

struct MyHttp;

impl Context for MyHttp {}

impl RootContext for TestWasm {
    fn on_vm_start(&mut self, _: usize) -> bool {
        info!("this is my  fist rust-wasm filter");
        true
    }
    fn create_http_context(&self, _context_id: u32) -> Option<Box<dyn HttpContext>> {
        Some(Box::new(MyHttp))
    }
    fn get_type(&self) -> Option<ContextType> {
        Some(ContextType::HttpContext)
    }
}

impl HttpContext for MyHttp {
    fn on_http_request_headers(&mut self, _num_headers: usize, _end_of_stream: bool) -> Action {
        match self.get_http_request_header(":path") {
            Some(path) => {
                info!("this is path:{}",path);
                Action::Continue
            }
            _ => Action::Continue
        }
    }
}
```

编译

```bash
cargo build --target wasm32-unknown-unknown --release
```

生成编译文件 `rustenvoy/target/wasm32-unknown-unknown/release/rustenvoy.wasm`

修改上文中的`envoy.yaml`为以下内容

```yaml
admin:
  address:
    socket_address: { address: 0.0.0.0, port_value: 9901 }

static_resources:

  listeners:
  - name: listener_0
    address:
      socket_address:
        address: 0.0.0.0
        port_value: 10000
    filter_chains:
    - filters:
      - name: envoy.filters.network.http_connection_manager
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          stat_prefix: ingress_http
          access_log:
          - name: envoy.access_loggers.stdout
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.access_loggers.stream.v3.StdoutAccessLog
          http_filters:

          - name: envoy.filters.http.wasm
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.wasm.v3.Wasm
              config:
                name: "rustfilter"
                root_id: "rust_wasm_id"
                configuration:
                  "@type": type.googleapis.com/google.protobuf.StringValue
                  value: |
                    {}
                vm_config:
                  runtime: "envoy.wasm.runtime.v8"
                  vm_id: "jwt"
                  code:
                    local:
                      filename: "/root/rustenvoy.wasm"
          - name: envoy.filters.http.router
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.router.v3.Router
          route_config:
            name: local_route
            virtual_hosts:
            - name: local_service
              domains: ["*"]
              routes:
              - match:
                  prefix: "/"
                route:
                  host_rewrite_literal: www.baidu.com
                  cluster: service_baidu_io

  clusters:
  - name: service_baidu_io
    type: LOGICAL_DNS
    # Comment out the following line to test on v6 networks
    dns_lookup_family: V4_ONLY
    load_assignment:
      cluster_name: service_baidu_io
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              socket_address:
                address: www.baidu.com
                port_value: 443
    transport_socket:
      name: envoy.transport_sockets.tls
      typed_config:
        "@type": type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext
        sni: www.baidu.com
```
如果出现以下报错,是说 `envoy.filters.http.router` 这个filter必须放到`http_filters`列表的最后一个
> 
> Error: terminal filter named envoy.filters.http.router of type envoy.filters.http.router must be the last filter in a http filter chain.
> 

重新启动访问测试

```
./envoy -c envoy.yaml
```

访问代理网址

```
curl http://192.168.50.233:10000/
```

查看日志

```bash
[2022-11-26 14:59:57.492][2524][info][wasm] [source/extensions/common/wasm/context.cc:1180] wasm log rust_wasm_id jwt: this is my  fist rust-wasm filter
[2022-11-26 15:00:00.530][2524][info][wasm] [source/extensions/common/wasm/context.cc:1180] wasm log rustfilter rust_wasm_id jwt: this is path:/
```

