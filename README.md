TODO:
* Send default tags with log message (hostname, ip)
* Filter on tags

## Sending a Log Stream
Every new TCP connection intended to stream logs must include a header as the first line.

#### Header Format
The first 7 characters must be `header:`. The remainder of the header is a series of semi-colon 
separated params used for various purposes, including but not limted to:
* Authenticating the request
* Formatting the stream output
* Filtering the stream output

`header:user=<username>;pass=<password>;source=<source>;app=<app>;proc=<proc>;dyno=<dyno>;hostname=<hostname>;env=<env>`

// We can format according to known app management services:
// Format:
// 2006-01-02T15:04:05.000 <host> <app>[<dyno>)]: <log>

// Generic:
log_message:
  app: ""
  proc: path.Base(os.Args[0])
  source: "generic"
  dyno: &proc.`pid`
  ip: <remote_ip>
  host: <remote_ip>
  log: <log>
// 2006-01-02T15:04:05.000 54.236.165.240 Kevins-MBP.fios-router.home[main.999]: <log>
// 2006-01-02T15:04:05.000 54.236.165.240 Kevins-MBP.fios-router.home[main.998]: <log>

// Docker Generic (`/.dockerenv` exists):
log_message:
  app: ""
  proc: path.Base(os.Args[0])
  source: "docker"
  dyno: &proc.`hostname`
  ip: <remote_ip>
  host: <remote_ip>
  log: <log>
// 2006-01-02T15:04:05.000 54.236.165.240 docker[worker.c12acf98a2bb]: <log>

// Heroku: (May not be possible without an official drain implementation)
log_message:
  app: $HEROKU_APP_NAME (depends on Proc Metadata lab feature)
  proc: strings.Split($DYNO, ".")[0]
  source: "heroku"
  dyno: $DYNO
  ip: <remote_ip>
  host: <remote_ip>
  log: <log>
// 2006-01-02T15:04:05.000 54.236.165.240 timehop[web.1]: <log>

// Docker Cloud:
log_message:
  app: $DOCKERCLOUD_STACK_NAME
  proc: $DOCKERCLOUD_SERVICE_HOSTNAME
  source: "docker-cloud"
  dyno: $DOCKERCLOUD_CONTAINER_HOSTNAME
  ip: <remote_ip>
  host: $DOCKERCLOUD_NODE_HOSTNAME
  log: <log>
// 2006-01-02T15:04:05.000 54.236.165.240 memories[memories-worker-facebook-3]: <log>

// EC2:
log_message:
  app: <Tag:Name>
  proc: path.Base(os.Args[0])
  source: "ec2"
  dyno: &proc.<instance_id>
  ip: <remote_ip>
  host: <remote_ip>
  log: <log>
// 2006-01-02T15:04:05.000 54.236.165.240 apicard-master[web.i-08351d06]: <log>
// 2006-01-02T15:04:05.000 54.236.165.240 i-10-12-10-24[web.i-08351d06]: <log>

// ECS:
// ?

// Tailing logstreams:
// Eg: To tail all memories workers EXCEPT the scheduler:
curl -N 'http://localhost:7575/tail?app=memories&proc=!/.+scheduler.+/'

// Eg: To tail rugen beta:
curl -N 'http://localhost:7575/tail?proc=/rugen-beta-.*/'


// Opening connection to send logs:
LOGIO_URL='logio://user:pass@127.0.0.1/logs?source=source&app=app&proc=proc&env=production'




