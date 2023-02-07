# m3u8-download

通过golang协程实现m3u8资源并行下载

### 介绍

1. 程序使用了golang自带的互斥锁`sync.Mutex`。
2. 有下载失败重试的逻辑，没有重试次数限制。
3. 下载完得到的是ts文件集，需要借助`ffmpet`合并为`mp4`
4. 脚本执行完后会给出合并ts文件为mp4的命令，类似：

    ```shell
    # 如果要执行这条命令，需要你的系统安装了ffmpeg软件命令
    ffmpeg -f concat -i download/merge.txt -c copy output.mp4
    ```

### 命令参数

1. `url` 通过m3u8的网络地址下载
2. `file` 通过m3u8文件本地路径下载，`url`和`file`二选一
3. `host` 如果配置了`file`参数，`host`将作为ts文件的服务器地址
4. `co` 开启的协程数，默认5，取值范围`(0,1000]`
5. `output` 文件下载后的存储目录