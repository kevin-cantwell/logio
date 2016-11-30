TODO:
* Send default tags with log message (hostname, ip)
* Filter on tags



// We can format according to known app management services:
// Generic:
// 2006-01-02T15:04:05.000 <ip> <app>[<proc>(.<proc_id>)]: <log>
// 2006-01-02T15:04:05.000 <ip> <`hostname`>[<`procname`>]: <log>
// 2006-01-02T15:04:05.000 <ip> ip-10-10-13-112[main]: <log>

// Docker Generic:
// 2006-01-02T15:04:05.000 <ip> <`hostname`>[<`procname`>]: <log>
// 2006-01-02T15:04:05.000 <ip> c12acf98a2bb[scheduler]: <log>

// Heroku: (May not be possible without an official drain implementation)
// 2006-01-02T15:04:05.000 <ip> <app>[<`proc`.`procid`>]: <log>
// 2006-01-02T15:04:05.000 <ip> timehop[web.1]: <log>

// Docker Cloud:
// 2006-01-02T15:04:05.000 <ip> <stack_name>[<service_name>.<container_inex>]: <log>
// 2006-01-02T15:04:05.000 <ip> memories[memories-worker-facebook.3]: <log>
// 2006-01-02T15:04:05.000 <ip> rugen[rugen-beta.1]: <log>

// EC2:
// 2006-01-02T15:04:05.000 <ip> <Tag:Name>[<`procname`.`Instance ID`>]: <log>
// 2006-01-02T15:04:05.000 <ip> apicard-master[web.i-08351d06]: <log>

// ECS:
// ?

// Tailing logstreams:
// Eg: To tail all memories workers EXCEPT the scheduler:
curl -N 'http://localhost:7575/tail?app=memories&proc=!/.+scheduler.+/'

// Eg: To tail rugen beta:
curl -N 'http://localhost:7575/tail?proc=/rugen-beta-.*/'