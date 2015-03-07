/* Package distshell is designed to manage simultaneous execution of commands against a cluster of nodes.
   DistShell has two methods for manipulating command execution behavior
   
   DistShell.monitor is modified by functions EnableMonitoring and DisableMonitoring.
   By default monitoring is enabled and will print host command status messages to stdout during execution.
   
   DistShell.maxBatch is modified by function SetMaxBatch.
   The default batch size is 50
 
  USAGE EXAMPLE
    hosts := "host1.localdomain,host2.localdomain,host3.localdomain"
    hlist := strings.Split(hosts, ",")
    shell := distshell.New(hlist)
    shell.AddCommand("host1.localdomain", "/bin/df")
    shell.AddCommand("host2.localdomain", "/bin/echo -n ''")
    shell.AddCommand("host3.localdomain", "/bin/echo 3")
    shell.Execute()
    shell.DumpAllStdout()
*/
package distshell

import (
    "fmt"
    "os/exec"
    "errors"
    "strings"
    "kit/kitutils"
    "os"
)

// Contains the hosts command information
type Host struct {
    Name string
    Stdout []byte
    cmd string  // no need to export
    args []string
    CmdError error
}

// Distshell uses static array of hosts for command execution 
type DistShell struct {
    HOSTS []Host
    monitor bool
    maxBatch int
}


// Build the host list and return the DistShell struct
func New(hList []string) *DistShell {
    ds := DistShell{buildHost(hList), true, 50}
    return &ds
}

// Build the host list and return the DistShell struct
func (ds *DistShell) SetupDistShell(hList []string) {
    ds.HOSTS = buildHost(hList)
    ds.EnableMonitoring()
    ds.SetMaxBatch(50)
}

// buildHost creates a list of host objects and returns from a list of hostnames
func buildHost(hList []string) []Host {
    hObj := make([]Host, len(hList))
    for i := range hList {
        h := Host{}
        h.Name = hList[i]
        hObj[i] = h
    }
    return hObj
}

// EnableMonitoring enables console output during command execution and is default behavior
func (ds *DistShell) EnableMonitoring() {
    ds.monitor = true
}

// DisableMonitoring disables console output during command execution
func (ds *DistShell) DisableMonitoring() {
    ds.monitor = false
}

// setMaxBatch modifies the max number of running go routines during command execution.  Default is 50
func (ds *DistShell) SetMaxBatch (n int) {
    ds.maxBatch = n
}

// add a command to a specific host
func (ds *DistShell) AddCommand(h string, command string, args ...string) bool {
    for i := range ds.HOSTS {
        if ds.HOSTS[i].Name == h {
            ds.HOSTS[i].cmd = command
            ds.HOSTS[i].args = args
            return true
        }
    }
    return false // if we made it here then this function failed
}

// Execute command string defined by all hosts and return comma delimited string of hosts that failed 
func (ds *DistShell) Execute() error {
    cmdStatus := make(chan string, ds.maxBatch)
    runningCount := 0
    TotalCmdsRun := 0
    TotalHosts := len(ds.HOSTS)
    for i := range ds.HOSTS {
        go runCMD(&ds.HOSTS[i], cmdStatus)
        runningCount += 1
        TotalCmdsRun += 1
        
        // we filled the batch or there are no more commands to run
        // so grab status for all running commands before
        if runningCount >= ds.maxBatch || TotalCmdsRun >= TotalHosts {
            for c := 0; c < runningCount; c++ {
                s := <-cmdStatus
                if ds.monitor {
                    fmt.Println(s)
                }
            }
            runningCount = 0
        }
    }
    
    // check for errors
    failedHosts := ""
    for i := range ds.HOSTS {
        if ds.HOSTS[i].CmdError != nil {
            failedHosts += ds.HOSTS[i].Name + ","   
        }
    }
    if failedHosts != "" {
        // trim the last comma and return comma delimited list of failed hosts
        failedHosts = strings.TrimRight(failedHosts, ",")
        return errors.New(failedHosts)
    }
    
    return nil
}

// ExecuteAll adds the given command to all hosts and executes.
func (ds *DistShell) ExecuteAll(cmd string, args ...string) error {
    for i := range ds.HOSTS {
        ds.AddCommand(ds.HOSTS[i].Name, cmd, args...)
    }
    if err := ds.Execute(); err != nil { return err }
    return nil
}

/* 
 *   GetFile will download a given file from remote node into specified dir
 *   filestring = /path/to/file
 *   destination = /path/to/destination/[dir|file]
 */
func (ds *DistShell) GetFile(filestring string, destination string) error {
    cmdStatus := make(chan string, ds.maxBatch)
    runningCount := 0
    TotalCmdsRun := 0
    TotalHosts := len(ds.HOSTS)
    
    SCP, lookupErr := exec.LookPath("scp")
    if lookupErr != nil {
        fmt.Printf("Unable to find scp in $PATH\n")
        os.Exit(1)
    }

    for i := range ds.HOSTS {
        go func(hostname *Host, cmdStatus chan string){
            remoteFile := hostname.Name + ":" + filestring
            cmdout, cmderr := kitutils.RunCMD(SCP, "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=no", remoteFile, destination)
            if cmderr != nil {
                hostname.CmdError = cmderr
                cmdStatus <-  fmt.Sprintf("%s: ERROR %s: %s", hostname.Name, cmdout, cmderr)
            } else {
                cmdStatus <- fmt.Sprintf("%s: SUCCESS", hostname.Name)
            }
        }(&ds.HOSTS[i], cmdStatus)
        runningCount += 1
        TotalCmdsRun += 1
        
        // we filled the batch or there are no more commands to run
        // so grab status for all running commands before
        if runningCount >= ds.maxBatch || TotalCmdsRun >= TotalHosts {
            for c := 0; c < runningCount; c++ {
                s := <-cmdStatus
                if ds.monitor {
                    fmt.Println(s)
                }
            }
            runningCount = 0
        }
    }
    
    // check for errors
    failedHosts := ""
    for i := range ds.HOSTS {
        if ds.HOSTS[i].CmdError != nil {
            failedHosts += ds.HOSTS[i].Name + ","   
        }
    }
    if failedHosts != "" {
        // trim the last comma and return comma delimited list of failed hosts
        failedHosts = strings.TrimRight(failedHosts, ",")
        return errors.New(failedHosts)
    }
    return nil
}

// Execute the command on the given remote host
func runCMD(h *Host, ch chan string ) {
    
    if h.cmd == "" {
        ch <- fmt.Sprintf("ERROR: host %s has no available command to execute", h.Name)
        h.CmdError = errors.New("no available command to execute")
    }

    SSH, lookupErr := exec.LookPath("ssh")
    if lookupErr != nil {
        fmt.Printf("Unable to find ssh in $PATH\n")
        os.Exit(1)
    }
    
    // build []string and ship it with exec.Command
    cmdArgs := make([]string, 0)
    cmdArgs = append(cmdArgs, "-o")
    cmdArgs = append(cmdArgs, "StrictHostKeyChecking=no")
    cmdArgs = append(cmdArgs, "-o")
    cmdArgs = append(cmdArgs, "BatchMode=yes")
    cmdArgs = append(cmdArgs, h.Name)
    cmdArgs = append(cmdArgs, h.cmd)
    for i := range h.args {
        cmdArgs = append(cmdArgs, h.args[i])
    }
    out, err := exec.Command(SSH, cmdArgs...).CombinedOutput()
    if err != nil {
        ch <- fmt.Sprintf("ERROR: Failed to exec command on host %s: %s", h.Name, err)
        h.Stdout = out
        h.CmdError = err
        return
    }
    h.Stdout = out    
    
    ch <- fmt.Sprintf("INFO: completed running command on host %s", h.Name)
    return
}

// print out the given hosts stdout
func (ds *DistShell) DumpHostStdout(h string) {
    for i := range ds.HOSTS {
        if ds.HOSTS[i].Name == h {
            fmt.Printf("Dumping output for cmd '%s' from host %s:\n%s", ds.HOSTS[i].cmd, ds.HOSTS[i].Name, ds.HOSTS[i].Stdout)
        }
    }
}

// print out the given hosts stdout
func (ds *DistShell) GetHostStdout(h string) []byte {
    for i := range ds.HOSTS {
        if ds.HOSTS[i].Name == h {
            return ds.HOSTS[i].Stdout
        }
    }
    return []byte{'n', 'o', ' ', 'o', 'u', 't', 'p', 'u', 't'}
}

// print stdout from all hosts
func (ds *DistShell) DumpAllStdout() {
    for i := range ds.HOSTS {
        fmt.Printf("Dumping output for host: %s\n%s", ds.HOSTS[i].Name, ds.HOSTS[i].Stdout)
    }
}

// runs command using exec and returns a string slice with command output
func RunCmdOutput(s string, arg ...string) ([]string, error) {
    execOut, err := exec.Command(s, arg...).Output()
    output := make([]string, 0)

    if err != nil {
        return output, &UtilError{"RunCmdOutput", errors.New(s + ": " + err.Error() + "\noutput:" + string(execOut))}
    }
    output = strings.Split(fmt.Sprintf("%s", execOut), "\n")

    if output[len(output)-1] == "\n" {
        return output[0 : len(output)-1], nil
    } else {
        return output, nil
    }
}

