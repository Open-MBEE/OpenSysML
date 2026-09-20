# OpenSysML SysON integration

This Maven reactor contains the OpenSysML backend adapter for SysON and a compile-only API stub
module. The default `stubs` profile builds without authenticated SysON artifacts; use
`-Psyson-artifacts` for the real Sirius Web and SysON dependencies.
