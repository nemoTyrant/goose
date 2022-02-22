## 功能

下载小鹅通视频。依赖`wget`和`ffmpeg`命令，macos可以使用`homebrew`安装。

## 使用方式

#### 1. 获取m3u8地址

微信打开视频页面后分享到电脑上，用chrome浏览器(或其他有开发者工具的浏览器)打开开发者工具，过滤`m3u8`，找到对应的链接。

#### 2. 使用工具下载

```shell
go build
./xiaoetong -u 'http://xxxx.m3u8' -n 新名称
```

#### 3. 文件数限制

分片数量过多时，ffmpeg拼接会报`Too many open files`。可以使用`ulimit -n`命令查看当前允许打开的最大数量。`ulimit -n 1024`可以修改最大数量(只对当前会话有效)。

## 实现原理

打开m3u8文件可以看到，视频是ts分片，并且使用aes-128方式加密。解析出密钥和所有ts分片链接后，使用`wget`命令下载。依次解密后使用`ffmpeg`命令拼接成完整视频。



## 参考资料

1.  [https://www.qinyuanyang.com/post/240.html](https://www.qinyuanyang.com/post/240.html)
2.  [https://www.qinyuanyang.com/post/247.html](https://www.qinyuanyang.com/post/247.html)

