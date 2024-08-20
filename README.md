# filewm
在线web文件管理器，适合本地个人使用，go语言低占用。

# 使用教程
环境:Debian12
```shell
apt update -y
apt install golang git -y
```
安装go环境，没有包的话请自行手动安装go环境。
```shell
git clone https://github.com/xkatld/filewm.git
cd filewm
chmod 777 filewm.go
go run filewm.go
```
默认设置38500端口，没有设置访问密码，先设置访问密码再切换是否启用密码进入。
修改端口：
```
	fmt.Println("Server is running on http://localhost:38500")
	log.Fatal(http.ListenAndServe(":38500", nil))
```
![image](https://github.com/user-attachments/assets/84cd60d9-4b0a-494c-ae2d-80de237865f7)

