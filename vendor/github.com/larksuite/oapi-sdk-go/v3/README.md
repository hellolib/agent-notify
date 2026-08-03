# 飞书开放接口SDK/Feishu OpenPlatform Server SDK

旨在让开发者便捷的调用飞书开放API、处理订阅的事件、处理服务端推送的卡片行为等。

Feishu Open Platform offers a series of server-side atomic APIs to achieve diverse functionalities. However, actual coding requires additional work, such as obtaining and maintaining access tokens, encrypting and decrypting data, and verifying request signatures. Furthermore, the lack of semantic descriptions for function calls and type system support can increase coding burdens.

To address these issues, Feishu Open Platform has developed the Open Interface SDK, which incorporates all lengthy logic processes, provides a comprehensive type system, and offers a semantic programming interface to enhance the coding experience.

## 介绍文档 Introduction Documents

- [开发前准备（安装） / Preparations before development(Install SDK)](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/server-side-sdk/golang-sdk-guide/preparations)
- [调用服务端 API / Calling Server-side APIs](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/server-side-sdk/golang-sdk-guide/calling-server-side-apis)
- [处理事件订阅 / Handle Events](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/server-side-sdk/golang-sdk-guide/handle-events)
- [处理卡片回调 / Handle Card Callbacks](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/server-side-sdk/golang-sdk-guide/handle-callback)
- [常见问题 / SDK FAQs](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/server-side-sdk/faq)

## 扩展示例

我们还基于 SDK 封装了常用的 API 组合调用及业务场景示例，如：

* 消息
    * [发送文件消息](https://github.com/larksuite/oapi-sdk-go-demo/blob/main/composite_api/im/send_file.go)
    * [发送图片消息](https://github.com/larksuite/oapi-sdk-go-demo/blob/main/composite_api/im/send_image.go)
* 通讯录
    * [获取部门下所有用户列表](https://github.com/larksuite/oapi-sdk-go-demo/blob/main/composite_api/contact/list_user_by_department.go)
* 多维表格
    * [创建多维表格同时添加数据表](https://github.com/larksuite/oapi-sdk-go-demo/blob/main/composite_api/base/create_app_and_tables.go)
* 电子表格
    * [复制粘贴某个范围的单元格数据](https://github.com/larksuite/oapi-sdk-go-demo/blob/main/composite_api/sheets/copy_and_paste_by_range.go)
    * [下载指定范围单元格的所有素材列表](https://github.com/larksuite/oapi-sdk-go-demo/blob/main/composite_api/sheets/download_media_by_range.go)
* 教程
    * [机器人自动拉群报警](https://github.com/larksuite/oapi-sdk-go-demo/blob/main/quick_start/robot) ([开发教程](https://open.feishu.cn/document/home/message-development-tutorial/introduction))

更多示例可参考：https://github.com/larksuite/oapi-sdk-go-demo

## 加入交流互助群

[单击加入交流互助](https://applink.feishu.cn/client/chat/chatter/add_by_link?link_token=985nb30c-787a-4fbb-904d-2cf945534078)

## License

使用 MIT



