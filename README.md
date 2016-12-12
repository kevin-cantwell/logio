TODO:
* Send default tags with log message (hostname, ip)
* Filter on tags

## Sending a Log Stream
Every new TCP connection intended to stream logs must include a header as the first line.


// I LIKE THIS ONE!
<timestamp> <app>[<proc>] <host>: <log>
2016-12-06T22:14:21.448 whois[cache-invalidator] whois-cache-invalidator-1: 2016/12/06 17:14:21 bar
2016-12-06T22:14:21.448 whois[web] i-10-34-20-12: 2016/12/06 17:14:21 bar
2016-12-06T22:14:21.448 whois[web] c12acf98a2bb: 2016/12/06 17:14:21 bar
2016-12-06T22:14:21.448 whois[web] Kevins-MBP.fios-router.home: 2016/12/06 17:14:21 bar

LOGIO_URL='logio://127.0.0.1?app=whois&id=$DOCKERCLOUD_CONTAINER_HOSTNAME'

LOGIO_URL='logio://user:pass@127.0.0.1/logs?source=source&app=app&proc=proc&env=production'




