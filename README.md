

 #USAGE EXAMPLE for unique commands per node
 ```
 func main() {
    hosts := "host1.localdomain,host2.localdomain,host3.localdomain"
    hlist := strings.Split(hosts, ",")
    shell := distshell.New(hlist)
    shell.AddCommand("host1.localdomain", "/bin/df")
    shell.AddCommand("host2.localdomain", "/bin/echo -n ''")
    shell.AddCommand("host3.localdomain", "/bin/echo 3")
    shell.Execute()
    shell.DumpAllStdout()
 }
 ```

 #USAGE EXAMPLE for running one command against all nodes
 ```
 func main(){
 	hosts := "host1.localdomain,host2.localdomain,host3.localdomain"
 	hlist := strings.Split(hosts, ",")
 	shell := distshell.New(hlist)
 	shell.ExecuteAll("echo", "hello")
 	shell.DumpAllStdout()
}
 ```