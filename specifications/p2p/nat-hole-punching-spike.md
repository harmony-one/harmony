# NAT Hole Punching Spike Report

(That is, can we run Harmony on a home network?)

## Purpose

Find out if we can easily punch a TCP port forwarding hole through a home gateway, so that we can run Harmony node at home.

## Goal

We should identify a way to easily tell the home gateway to punch a TCP hole so that it is forwarded to a local IP address and port number of our choice, that is, we configure our Harmony client to run on that address/port.

## Background

VoIP and games have needed this for a long time, and there are a few protocols that can do this:

* [NAT Port Mapping Protocol (NAT-PMP)](https://tools.ietf.org/html/rfc6886)
    * Apple’s version
    * Used by Back To My Mac
    * IETF Informational (recognized but not standardized)
* [Port Control Protocol (PCP)](https://tools.ietf.org/html/rfc6887)
    * Successor of NAT-PMP
    * Works with large-scale NAT
    * IETF Proposed Standard
* Internet Gateway Device (IGD) Control Protocol
    * Microsoft’s version, originally part of Universal Plug and Play (UPnP)
        * UPnP is now ISO/IEC 29341
    * Older, dates back to first Xbox (gaming was the first use case)

Most home gateways support one or both of these.  IGD/UPnP is more common because it’s older and the use case is more compelling – PC/console gaming.  We first focus on IGD therefore.  Our office is behind a Google Wi-Fi router, which supports both IGD and PCP.

## Plan & “tl;dr” Result

(See Transcript for details.)

1. Find a CLI-based IGD/UPnP implementation we can use in our scripts.<br/>**⇒ MiniUPnP seemed promising**
2. Examine docs to see if it can setup and teardown the port mapping.<br/>**⇒ list (upnpc -l), setup (upnp -r), teardown (upnp -d)**
3. Set up a port forwarding to SSH daemon my Mac<br/>**⇒ upnpc -r 22 TCP**
4. Figure out the port mapping established.<br/>**⇒ found in the upnpc -r command output, also shown in upnpc -l output**
5. SSH to a server out of the home network.
6. SSH back to my Mac, using the public IP/port obtained from step 4.<br/>**⇒ WORKS!**
7. Tear down the port mapping.<br/>**⇒ upnpc -d 22 TCP**

## Next Step

1. Use this for the startup script to use in Docker.
2. See if we can also find a suitable CLI tool for PCP or NAT-PMP.
    a. I believe Apple AirPort access point implements only NAT-PMP, and not UPnP.
3. Figure out how to deal with timeout issue.
    a. Can we refresh existing mapping so the public port number does not change?
4. Figure out how to deal with changes in public IP address.
    a. When this happens, public port may also change, or mapping may be gone.
5. Figure out how to deal with router reboots.
    a. Depends on implementation, but chances are, the mapping may be gone.

Also, **everyone:**

6. Try this at home.
    a. Let me know how it goes.

## Appendix A. Transcript

### Docs Examination

Command-line help; bold ones seem to fit our purposes.
```
quelthalas 09:07:25 ~ $ 11 upnpc -h
upnpc : miniupnpc library test client, version 2.1.
 (c) 2005-2018 Thomas Bernard.
Go to http://miniupnp.free.fr/ or https://miniupnp.tuxfamily.org/
for more information.
Usage :	upnpc [options] -a ip port external_port protocol [duration]
		Add port redirection
       	upnpc [options] -d external_port protocol <remote host>
		Delete port redirection
       	upnpc [options] -s
		Get Connection status
       	upnpc [options] -l
		List redirections
       	upnpc [options] -L
		List redirections (using GetListOfPortMappings (for IGD:2 only)
       	upnpc [options] -n ip port external_port protocol [duration]
		Add (any) port redirection allowing IGD to use alternative external_port (for IGD:2 only)
       	upnpc [options] -N external_port_start external_port_end protocol [manage]
		Delete range of port redirections (for IGD:2 only)
       	upnpc [options] -r port1 [external_port1] protocol1 [port2 [external_port2] protocol2] [...]
		Add all redirections to the current host
       	upnpc [options] -A remote_ip remote_port internal_ip internal_port protocol lease_time
		Add Pinhole (for IGD:2 only)
       	upnpc [options] -U uniqueID new_lease_time
		Update Pinhole (for IGD:2 only)
       	upnpc [options] -C uniqueID
		Check if Pinhole is Working (for IGD:2 only)
       	upnpc [options] -K uniqueID
		Get Number of packets going through the rule (for IGD:2 only)
       	upnpc [options] -D uniqueID
		Delete Pinhole (for IGD:2 only)
       	upnpc [options] -S
		Get Firewall status (for IGD:2 only)
       	upnpc [options] -G remote_ip remote_port internal_ip internal_port protocol
		Get Outbound Pinhole Timeout (for IGD:2 only)
       	upnpc [options] -P
		Get Presentation url

protocol is UDP or TCP
Options:
  -e description : set description for port mapping.
  -6 : use ip v6 instead of ip v4.
  -u url : bypass discovery process by providing the XML root description url.
  -m address/interface : provide ip address (ip v4) or interface name (ip v4 or v6) to use for sending SSDP multicast packets.
  -z localport : SSDP packets local (source) port (1024-65535).
  -p path : use this path for MiniSSDPd socket.
  -t ttl : set multicast TTL. Default value is 2.
```

## Forwarding Setup

### Precondition

```
quelthalas 09:07:27 ~ $ 12 upnpc -l
upnpc : miniupnpc library test client, version 2.1.
 (c) 2005-2018 Thomas Bernard.
Go to http://miniupnp.free.fr/ or https://miniupnp.tuxfamily.org/
for more information.
List of UPNP devices found on the network :
 desc: http://192.168.86.1:5000/rootDesc.xml
 st: urn:schemas-upnp-org:device:InternetGatewayDevice:1

Found valid IGD : http://192.168.86.1:5000/ctl/IPConn
Local LAN ip address : 192.168.86.250
Connection Type : IP_Routed
Status : Connected, uptime=4460733s, LastConnectionError : ERROR_NONE
  Time started : Wed Jan 16 18:02:03 2019
MaxBitRateDown : 1000000000 bps (1000.0 Mbps)   MaxBitRateUp 1000000000 bps (1000.0 Mbps)
ExternalIPAddress = 73.71.156.214
 i protocol exPort->inAddr:inPort description remoteHost leaseTime
 0 UDP 58956->192.168.86.230:58956 'WhatsApp (1552097793) ()' '' 551337
 1 UDP 57027->192.168.86.230:57027 'WhatsApp (1552098628) ()' '' 552173
 2 UDP 58671->192.168.86.230:58671 'WhatsApp (1552149239) ()' '' 602783
 3 UDP 55355->192.168.86.230:55355 'WhatsApp (1552150382) ()' '' 603926
GetGenericPortMappingEntry() returned 713 (SpecifiedArrayIndexInvalid)
```

### Trigger

```
quelthalas 09:07:36 ~ $ 13 upnpc -r 22 TCP
upnpc : miniupnpc library test client, version 2.1.
 (c) 2005-2018 Thomas Bernard.
Go to http://miniupnp.free.fr/ or https://miniupnp.tuxfamily.org/
for more information.
List of UPNP devices found on the network :
 desc: http://192.168.86.1:5000/rootDesc.xml
 st: urn:schemas-upnp-org:device:InternetGatewayDevice:1

Found valid IGD : http://192.168.86.1:5000/ctl/IPConn
Local LAN ip address : 192.168.86.250
ExternalIPAddress = 73.71.156.214
InternalIP:Port = 192.168.86.250:22
external 73.71.156.214:22 TCP is redirected to internal 192.168.86.250:22 (duration=604800)
```

It assigned the external port number (exPort) 22.  The external IP address is 73.71.156.214.  It also has a one-week (604800 seconds) lifetime.

### Postcondition

```
quelthalas 09:07:41 ~ $ 14 upnpc -l
upnpc : miniupnpc library test client, version 2.1.
 (c) 2005-2018 Thomas Bernard.
Go to http://miniupnp.free.fr/ or https://miniupnp.tuxfamily.org/
for more information.
List of UPNP devices found on the network :
 desc: http://192.168.86.1:5000/rootDesc.xml
 st: urn:schemas-upnp-org:device:InternetGatewayDevice:1

Found valid IGD : http://192.168.86.1:5000/ctl/IPConn
Local LAN ip address : 192.168.86.250
Connection Type : IP_Routed
Status : Connected, uptime=4460745s, LastConnectionError : ERROR_NONE
  Time started : Wed Jan 16 18:02:03 2019
MaxBitRateDown : 1000000000 bps (1000.0 Mbps)   MaxBitRateUp 1000000000 bps (1000.0 Mbps)
ExternalIPAddress = 73.71.156.214
 i protocol exPort->inAddr:inPort description remoteHost leaseTime
 0 UDP 58956->192.168.86.230:58956 'WhatsApp (1552097793) ()' '' 551325
 1 UDP 57027->192.168.86.230:57027 'WhatsApp (1552098628) ()' '' 552161
 2 UDP 58671->192.168.86.230:58671 'WhatsApp (1552149239) ()' '' 602771
 3 UDP 55355->192.168.86.230:55355 'WhatsApp (1552150382) ()' '' 603914
 4 TCP    22->192.168.86.250:22    'libminiupnpc' '' 604793
GetGenericPortMappingEntry() returned 713 (SpecifiedArrayIndexInvalid)
```

Index 4 is the new entry we just set up.

### SSH Roundtrip Test

#### SSH to an out-of-home server

We use our Jenkins server on AWS.

```
quelthalas 09:07:48 ~ $ 15 ssh jenkins
Last login: Sat Mar  9 16:45:50 2019 from c-73-71-156-214.hsd1.ca.comcast.net

       __|  __|_  )
       _|  (     /   Amazon Linux 2 AMI
      ___|\___|___|

https://aws.amazon.com/amazon-linux-2/
8 package(s) needed for security, out of 11 available
Run "sudo yum update" to apply all updates.
[ec2-user@ip-172-31-14-35 ~]$ 
```

#### SSH back to my Mac

Use the external address and port number we gathered.

```
[ec2-user@ip-172-31-14-35 ~]$ ssh -p 22 ek@73.71.156.214
Password:
Last login: Sat Mar  9 08:46:14 2019 from 54.183.5.66
Agent running at /var/folders/bj/fgmkk8fj715_jcbdzrxjlrh80000gn/T//ssh-73WRpgJuix8J/agent.9152 (PID 9153)
quelthalas 09:08:13 ~ $ 1 logout
Connection to 73.71.156.214 closed.
[ec2-user@ip-172-31-14-35 ~]$ logout
Shared connection to jenkins.harmony.one closed.
quelthalas 09:08:30 ~ $ 16 
```

### Forwarding Teardown

#### Trigger

```
quelthalas 09:29:12 ~ $ 18 upnpc -d 22 TCP
upnpc : miniupnpc library test client, version 2.1.
 (c) 2005-2018 Thomas Bernard.
Go to http://miniupnp.free.fr/ or https://miniupnp.tuxfamily.org/
for more information.
List of UPNP devices found on the network :
 desc: http://192.168.86.1:5000/rootDesc.xml
 st: urn:schemas-upnp-org:device:InternetGatewayDevice:1

Found valid IGD : http://192.168.86.1:5000/ctl/IPConn
Local LAN ip address : 192.168.86.250
UPNP_DeletePortMapping() returned : 0
```

#### Postcondition

```
quelthalas 09:29:19 ~ $ 19 upnpc -l
upnpc : miniupnpc library test client, version 2.1.
 (c) 2005-2018 Thomas Bernard.
Go to http://miniupnp.free.fr/ or https://miniupnp.tuxfamily.org/
for more information.
List of UPNP devices found on the network :
 desc: http://192.168.86.1:5000/rootDesc.xml
 st: urn:schemas-upnp-org:device:InternetGatewayDevice:1

Found valid IGD : http://192.168.86.1:5000/ctl/IPConn
Local LAN ip address : 192.168.86.250
Connection Type : IP_Routed
Status : Connected, uptime=4462039s, LastConnectionError : ERROR_NONE
  Time started : Wed Jan 16 18:02:03 2019
MaxBitRateDown : 1000000000 bps (1000.0 Mbps)   MaxBitRateUp 1000000000 bps (1000.0 Mbps)
ExternalIPAddress = 73.71.156.214
 i protocol exPort->inAddr:inPort description remoteHost leaseTime
 0 UDP 58956->192.168.86.230:58956 'WhatsApp (1552097793) ()' '' 550031
 1 UDP 57027->192.168.86.230:57027 'WhatsApp (1552098628) ()' '' 550867
 2 UDP 58671->192.168.86.230:58671 'WhatsApp (1552149239) ()' '' 601477
GetGenericPortMappingEntry() returned 713 (SpecifiedArrayIndexInvalid)
```

Note that the TCP port 22 entry is gone.  Teardown worked.
