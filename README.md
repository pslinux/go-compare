# go-compare

go配置文件对比application.properties完成更新

go build -o update_config-application.properties-v2.2 update_config-application.properties-v2.2.go

./update_config-application.properties-v2.2

配置文件更新工具 v1.1.0 (构建日期: 2023-11-20)
用法: ./update_config-application.properties-v2.2 [选项] 旧配置文件路径 新配置文件路径

选项:
  -v    启用详细输出模式
  -version
        显示版本信息

示例:
  ./update_config-application.properties-v2.2 old.properties new.properties
  ./update_config-application.properties-v2.2 -v old.properties new.properties

#config-matcher.json

- 用于在配文件当中定义新增的配置选项
- 如果是update_config-application.properties-v2.2.go当中没有包含的配置参数
