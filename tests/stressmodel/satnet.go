// Package stressmodel generates large SysML v2 models of a stated shape and size,
// so the toolchain's cost can be measured against a model that looks like one a
// systems engineer would write rather than against one repeated declaration.
//
// The satellite network is a constellation of fully modeled spacecraft: every
// satellite declares its own subsystems and their components with as-built
// attribute values, the power and data connections between them, a mass and a
// power budget, the requirements it satisfies and the mode machine it exhibits.
// Satellites are cross-linked to their neighbours in the same orbital plane and
// in the adjacent plane, and every satellite has a downlink to a ground station.
package stressmodel

import (
	"fmt"
	"io"
	"strings"
)

// SatelliteNetwork sizes the generated constellation.
type SatelliteNetwork struct {
	// Planes is the number of orbital planes; Satellites is the number in each.
	Planes, Satellites int
	// GroundStations is the number of ground stations the constellation downlinks to.
	GroundStations int
}

// Stats counts what a generated network declares.
type Stats struct {
	Satellites, GroundStations int
	// Components is the number of leaf components across every satellite and station.
	Components int
	// Connections is the number of interface usages, inside satellites and between them.
	Connections int
	// Requirements is the number of requirement usages, each with a satisfy assertion.
	Requirements int
	// Elements is the number of declarations the model makes, library included.
	Elements int
	// Bytes is the length of the generated source.
	Bytes int
}

// subsystems are what every satellite carries, with the components of each.
var subsystems = []struct {
	name, def  string
	components []struct{ name, def string }
}{
	{"eps", "ElectricalPowerSubsystem", []struct{ name, def string }{
		{"solarArray", "SolarArray"}, {"battery", "Battery"}, {"pcdu", "PowerConditioner"}}},
	{"adcs", "AttitudeControlSubsystem", []struct{ name, def string }{
		{"starTracker", "StarTracker"}, {"imu", "InertialMeasurementUnit"},
		{"wheelX", "ReactionWheel"}, {"wheelY", "ReactionWheel"}, {"wheelZ", "ReactionWheel"}, {"wheelS", "ReactionWheel"}}},
	{"cdh", "CommandAndDataHandling", []struct{ name, def string }{
		{"obc", "OnboardComputer"}, {"massMemory", "MassMemory"}}},
	{"comms", "CommunicationsSubsystem", []struct{ name, def string }{
		{"transponder", "Transponder"}, {"crosslinkTerminal", "CrosslinkTerminal"}, {"antenna", "Antenna"}}},
	{"prop", "PropulsionSubsystem", []struct{ name, def string }{
		{"tank", "PropellantTank"}, {"thruster", "Thruster"}}},
	{"thermal", "ThermalSubsystem", []struct{ name, def string }{
		{"radiator", "Radiator"}, {"heater", "Heater"}}},
	{"payload", "Payload", []struct{ name, def string }{
		{"sensor", "ImagingSensor"}, {"processor", "PayloadProcessor"}}},
}

// stationComponents are what every ground station carries.
var stationComponents = []struct{ name, def string }{
	{"dish", "Antenna"}, {"modem", "Transponder"}, {"server", "OnboardComputer"}, {"ups", "Battery"},
}

// Generate writes the network to w and reports what it declared.
func (n SatelliteNetwork) Generate(w io.Writer) (Stats, error) {
	var b strings.Builder
	g := &generator{b: &b}
	g.library()
	g.constellation(n)
	g.stats.Bytes = b.Len()
	_, err := io.WriteString(w, b.String())
	return g.stats, err
}

// Source returns the generated network as a string.
func (n SatelliteNetwork) Source() (string, Stats) {
	var b strings.Builder
	stats, _ := n.Generate(&b)
	return b.String(), stats
}

type generator struct {
	b     *strings.Builder
	stats Stats
}

// decl writes one declaration line at the given depth and counts it.
func (g *generator) decl(depth int, format string, args ...any) {
	g.stats.Elements++
	g.line(depth, format, args...)
}

func (g *generator) line(depth int, format string, args ...any) {
	for i := 0; i < depth; i++ {
		g.b.WriteString("    ")
	}
	fmt.Fprintf(g.b, format, args...)
	g.b.WriteByte('\n')
}

// imports writes a private import of each named package's members.
func (g *generator) imports(depth int, packages ...string) {
	for _, p := range packages {
		g.line(depth, "private import %s::*;", p)
	}
}

// redefinePart opens the body of a part usage redefining name.
func (g *generator) redefinePart(depth int, name string) {
	g.decl(depth, "part :>> %s {", name)
}

// dataRate redefines a link's or terminal's data rate.
func (g *generator) dataRate(depth, rate int) {
	g.decl(depth, "attribute :>> dataRate = %d.0;", rate)
}

// spacecraftSubject writes the subject every spacecraft requirement constrains.
func (g *generator) spacecraftSubject(depth int) {
	g.decl(depth, "subject sc : Spacecraft;")
}

// library writes the fixed definitions every satellite is built from.
func (g *generator) library() {
	g.decl(0, "package SatelliteNetwork {")
	g.imports(1, "ScalarValues", "ISQ", "SI")
	g.line(0, "")
	g.decl(1, "package Items {")
	g.decl(2, "item def Telemetry { attribute timestamp :> ISQ::time; attribute frameBytes : Integer; }")
	g.stats.Elements += 2
	g.decl(2, "item def Command { attribute opcode : Integer; attribute argument : Integer; }")
	g.stats.Elements += 2
	g.decl(2, "item def ElectricalPower { attribute watts :> ISQ::power; }")
	g.stats.Elements++
	g.decl(2, "item def ScienceData { attribute frames : Integer; attribute compressed : Boolean; }")
	g.stats.Elements += 2
	g.line(1, "}")
	g.line(0, "")
	g.decl(1, "package Ports {")
	g.imports(2, "Items")
	g.decl(2, "port def RFPort { out item tlmOut : Telemetry; in item cmdIn : Command; }")
	g.stats.Elements += 2
	g.decl(2, "port def PowerOut { out item power : ElectricalPower; }")
	g.stats.Elements++
	g.decl(2, "port def PowerIn { in item power : ElectricalPower; }")
	g.stats.Elements++
	g.decl(2, "port def DataOut { out item data : ScienceData; }")
	g.stats.Elements++
	g.decl(2, "port def DataIn { in item data : ScienceData; }")
	g.stats.Elements++
	g.decl(2, "port def BusMaster { out item cmd : Command; in item tlm : Telemetry; }")
	g.stats.Elements += 2
	g.decl(2, "port def BusNode { in item cmd : Command; out item tlm : Telemetry; }")
	g.stats.Elements += 2
	g.line(1, "}")
	g.line(0, "")
	g.decl(1, "package Interfaces {")
	g.imports(2, "Ports")
	g.decl(2, "interface def RFLink {")
	g.decl(3, "end a : RFPort;")
	g.decl(3, "end b : ~RFPort;")
	g.decl(3, "attribute dataRate : Real;")
	g.decl(3, "attribute slantRange :> ISQ::length;")
	g.decl(3, "flow a.tlmOut to b.tlmOut;")
	g.decl(3, "flow b.cmdIn to a.cmdIn;")
	g.line(2, "}")
	g.decl(2, "interface def PowerFeed {")
	g.decl(3, "end supply : PowerOut;")
	g.decl(3, "end load : PowerIn;")
	g.decl(3, "attribute busVoltage :> ISQ::electricPotential;")
	g.decl(3, "flow supply.power to load.power;")
	g.line(2, "}")
	g.decl(2, "interface def DataBusLink {")
	g.decl(3, "end master : BusMaster;")
	g.decl(3, "end node : BusNode;")
	g.decl(3, "flow master.cmd to node.cmd;")
	g.decl(3, "flow node.tlm to master.tlm;")
	g.line(2, "}")
	g.decl(2, "interface def DataFeed {")
	g.decl(3, "end source : DataOut;")
	g.decl(3, "end sink : DataIn;")
	g.decl(3, "flow source.data to sink.data;")
	g.line(2, "}")
	g.line(1, "}")
	g.line(0, "")
	g.decl(1, "package Components {")
	g.imports(2, "Ports")
	g.decl(2, "abstract part def Component {")
	g.decl(3, "attribute mass :> ISQ::mass;")
	g.decl(3, "attribute powerDraw :> ISQ::power;")
	g.decl(3, "attribute serialNumber : String;")
	g.decl(3, "port power : PowerIn;")
	g.decl(3, "port bus : BusNode;")
	g.decl(3, "constraint massIsPositive { mass > 0 [kg] }")
	g.line(2, "}")
	g.decl(2, "part def SolarArray :> Component { attribute area :> ISQ::area; attribute generated :> ISQ::power; port supply : PowerOut; }")
	g.stats.Elements += 3
	g.decl(2, "part def Battery :> Component { attribute capacity :> ISQ::energy; attribute depthOfDischarge : Real; port supply : PowerOut; }")
	g.stats.Elements += 3
	g.decl(2, "part def PowerConditioner :> Component { attribute efficiency : Real; port supply : PowerOut; port arrayIn : PowerIn; port batteryIn : PowerIn; }")
	g.stats.Elements += 4
	g.decl(2, "part def StarTracker :> Component { attribute accuracy :> ISQ::planeAngle; attribute updateRate :> ISQ::frequency; }")
	g.stats.Elements += 2
	g.decl(2, "part def InertialMeasurementUnit :> Component { attribute driftRate : Real; }")
	g.stats.Elements++
	g.decl(2, "part def ReactionWheel :> Component { attribute maxTorque :> ISQ::torque; attribute momentumCapacity : Real; }")
	g.stats.Elements += 2
	g.decl(2, "part def OnboardComputer :> Component { attribute clockRate :> ISQ::frequency; attribute memoryBytes : Integer; port bus1 : BusMaster; port bus2 : BusMaster; port bus3 : BusMaster; port bus4 : BusMaster; port bus5 : BusMaster; port bus6 : BusMaster; port dataIn : DataIn; }")
	g.stats.Elements += 9
	g.decl(2, "part def MassMemory :> Component { attribute capacityBytes : Integer; port dataIn : DataIn; }")
	g.stats.Elements += 2
	g.decl(2, "part def Transponder :> Component { attribute frequency :> ISQ::frequency; attribute transmitPower :> ISQ::power; port rf : RFPort; }")
	g.stats.Elements += 3
	g.decl(2, "part def CrosslinkTerminal :> Component { attribute wavelength :> ISQ::length; attribute dataRate : Real; port tx : RFPort; port rx : ~RFPort; }")
	g.stats.Elements += 4
	g.decl(2, "part def Antenna :> Component { attribute gain : Real; attribute diameter :> ISQ::length; }")
	g.stats.Elements += 2
	g.decl(2, "part def PropellantTank :> Component { attribute propellantMass :> ISQ::mass; attribute volume :> ISQ::volume; }")
	g.stats.Elements += 2
	g.decl(2, "part def Thruster :> Component { attribute thrust :> ISQ::force; attribute specificImpulse :> ISQ::time; }")
	g.stats.Elements += 2
	g.decl(2, "part def Radiator :> Component { attribute area :> ISQ::area; attribute emissivity : Real; }")
	g.stats.Elements += 2
	g.decl(2, "part def Heater :> Component { attribute setpoint :> ISQ::temperature; }")
	g.stats.Elements++
	g.decl(2, "part def ImagingSensor :> Component { attribute groundSampleDistance :> ISQ::length; attribute swath :> ISQ::length; port dataOut : DataOut; }")
	g.stats.Elements += 3
	g.decl(2, "part def PayloadProcessor :> Component { attribute throughput : Real; port dataIn : DataIn; port dataOut : DataOut; }")
	g.stats.Elements += 3
	g.line(1, "}")
	g.line(0, "")
	g.decl(1, "package Subsystems {")
	g.imports(2, "Ports", "Interfaces", "Components")
	g.decl(2, "abstract part def Subsystem {")
	g.decl(3, "attribute mass :> ISQ::mass;")
	g.decl(3, "attribute powerDraw :> ISQ::power;")
	g.decl(3, "port power : PowerIn;")
	g.decl(3, "port bus : BusNode;")
	g.line(2, "}")
	for _, s := range subsystems {
		g.decl(2, "part def %s :> Subsystem {", s.def)
		for _, c := range s.components {
			g.decl(3, "part %s : %s;", c.name, c.def)
		}
		switch s.name {
		case "eps":
			g.decl(3, "port supply : PowerOut;")
			g.decl(3, "interface arrayToPcdu : PowerFeed connect solarArray.supply to pcdu.arrayIn;")
			g.decl(3, "interface batteryToPcdu : PowerFeed connect battery.supply to pcdu.batteryIn;")
			g.decl(3, "bind pcdu.supply = supply;")
		case "cdh":
			g.decl(3, "port dataIn : DataIn;")
			g.decl(3, "interface obcToMemory : DataBusLink connect obc.bus1 to massMemory.bus;")
			g.decl(3, "bind obc.dataIn = dataIn;")
		case "comms":
			g.decl(3, "port rf : RFPort;")
			g.decl(3, "port crosslinkTx : RFPort;")
			g.decl(3, "port crosslinkRx : ~RFPort;")
			g.decl(3, "bind transponder.rf = rf;")
			g.decl(3, "bind crosslinkTerminal.tx = crosslinkTx;")
			g.decl(3, "bind crosslinkTerminal.rx = crosslinkRx;")
		case "payload":
			g.decl(3, "port dataOut : DataOut;")
			g.decl(3, "interface sensorToProcessor : DataFeed connect sensor.dataOut to processor.dataIn;")
			g.decl(3, "bind processor.dataOut = dataOut;")
		}
		g.line(2, "}")
	}
	g.line(1, "}")
	g.line(0, "")
	g.decl(1, "package Behavior {")
	g.decl(2, "attribute def Recovered;")
	g.decl(2, "attribute def ImagingWindow;")
	g.decl(2, "attribute def GroundContact;")
	g.decl(2, "attribute def PassComplete;")
	g.decl(2, "attribute def Fault;")
	g.decl(2, "state def SpacecraftModes {")
	g.decl(3, "attribute passes : Integer = 0;")
	g.decl(3, "entry; then safe;")
	g.decl(3, "state safe;")
	g.decl(3, "state nominal;")
	g.decl(3, "state imaging;")
	g.decl(3, "state downlinking {")
	g.decl(4, "entry assign passes := passes + 1;")
	g.line(3, "}")
	g.decl(3, "transition first safe accept Recovered then nominal;")
	g.decl(3, "transition first nominal accept ImagingWindow then imaging;")
	g.decl(3, "transition first imaging accept GroundContact then downlinking;")
	g.decl(3, "transition first downlinking accept PassComplete then nominal;")
	g.decl(3, "transition first nominal accept Fault then safe;")
	g.decl(3, "transition first imaging accept Fault then safe;")
	g.decl(3, "transition first downlinking accept Fault then safe;")
	g.line(2, "}")
	g.decl(2, "calc def LinkMargin {")
	g.decl(3, "in transmitPower :> ISQ::power;")
	g.decl(3, "in gain : Real;")
	g.decl(3, "in slantRange :> ISQ::length;")
	g.decl(3, "return margin : Real = gain * transmitPower / (slantRange * slantRange);")
	g.line(2, "}")
	g.decl(2, "action def DownlinkPass {")
	g.decl(3, "in frames : Integer;")
	g.decl(3, "out delivered : Integer = frames;")
	g.line(2, "}")
	g.line(1, "}")
	g.line(0, "")
	g.decl(1, "package Requirements {")
	g.imports(2, "Platform")
	g.decl(2, "requirement def MassBudget {")
	g.spacecraftSubject(3)
	g.decl(3, "attribute limit :> ISQ::mass;")
	g.decl(3, "require constraint { sc.dryMass <= limit }")
	g.line(2, "}")
	g.decl(2, "requirement def PowerBudget {")
	g.spacecraftSubject(3)
	g.decl(3, "require constraint { sc.totalPowerDraw <= sc.eps.solarArray.generated }")
	g.line(2, "}")
	g.decl(2, "requirement def CrosslinkCapacity {")
	g.spacecraftSubject(3)
	g.decl(3, "attribute minimumRate : Real;")
	g.decl(3, "require constraint { sc.comms.crosslinkTerminal.dataRate >= minimumRate }")
	g.line(2, "}")
	g.line(1, "}")
	g.line(0, "")
	g.decl(1, "package Platform {")
	g.imports(2, "Ports", "Interfaces", "Subsystems", "Behavior")
	g.decl(2, "abstract part def Spacecraft {")
	g.decl(3, "attribute catalogId : Integer;")
	g.decl(3, "attribute plane : Integer;")
	g.decl(3, "attribute slot : Integer;")
	g.decl(3, "attribute dryMass :> ISQ::mass;")
	g.decl(3, "attribute totalPowerDraw :> ISQ::power;")
	for _, s := range subsystems {
		g.decl(3, "part %s : %s;", s.name, s.def)
	}
	g.decl(3, "exhibit state modes : SpacecraftModes;")
	g.line(2, "}")
	g.decl(2, "part def GroundStation {")
	g.decl(3, "attribute stationId : Integer;")
	g.decl(3, "attribute latitude : Real;")
	g.decl(3, "attribute longitude : Real;")
	for _, c := range stationComponents {
		g.decl(3, "part %s : Components::%s;", c.name, c.def)
	}
	g.decl(3, "port uplink : ~RFPort;")
	g.decl(3, "bind modem.rf = uplink;")
	g.decl(3, "interface serverToModem : DataBusLink connect server.bus1 to modem.bus;")
	g.decl(3, "interface upsToServer : PowerFeed connect ups.supply to server.power;")
	g.line(2, "}")
	g.line(1, "}")
	g.line(0, "")
}

// constellation writes every satellite and ground station and the links between them.
func (g *generator) constellation(n SatelliteNetwork) {
	g.decl(1, "package Constellation {")
	g.imports(2, "Interfaces", "Platform", "Requirements", "Behavior")
	id := 0
	for p := 0; p < n.Planes; p++ {
		for s := 0; s < n.Satellites; s++ {
			g.satellite(id, p, s)
			id++
		}
	}
	for k := 0; k < n.GroundStations; k++ {
		g.groundStation(k)
	}
	g.network(n)
	g.line(1, "}")
	g.line(0, "}")
}

// network writes the part definition joining every satellite and ground
// station, and its one usage.
func (g *generator) network(n SatelliteNetwork) {
	g.line(0, "")
	g.decl(2, "part def Network {")
	total := n.Planes * n.Satellites
	for i := 0; i < total; i++ {
		g.decl(3, "part sat%d : Sat%d;", i, i)
	}
	for k := 0; k < n.GroundStations; k++ {
		g.decl(3, "part gs%d : Station%d;", k, k)
	}
	for p := 0; p < n.Planes; p++ {
		for s := 0; s < n.Satellites; s++ {
			i := p*n.Satellites + s
			if n.Satellites > 2 || (n.Satellites == 2 && s == 0) {
				g.link("ring", i, p*n.Satellites+(s+1)%n.Satellites)
			}
			if p+1 < n.Planes {
				g.link("plane", i, (p+1)*n.Satellites+s)
			}
			if n.GroundStations > 0 {
				k := i % n.GroundStations
				g.stats.Connections++
				g.decl(3, "interface downlink%dTo%d : RFLink connect sat%d.comms.rf to gs%d.uplink {", k, i, i, k)
				g.dataRate(4, 50+i%200)
				g.decl(4, "attribute :>> slantRange = %d [km];", 900+i%1500)
				g.line(3, "}")
			}
		}
	}
	g.decl(3, "attribute satelliteCount : Integer = %d;", total)
	g.line(2, "}")
	g.decl(2, "part network : Network;")
}

// link writes a crosslink between two satellites' crosslink terminals.
func (g *generator) link(kind string, a, b int) {
	g.stats.Connections++
	g.decl(3, "interface %s%dTo%d : RFLink connect sat%d.comms.crosslinkTx to sat%d.comms.crosslinkRx {", kind, a, b, a, b)
	g.dataRate(4, 100+(a+b)%400)
	g.decl(4, "attribute :>> slantRange = %d [km];", 2000+(a*7+b*3)%3000)
	g.line(3, "}")
}

// satellite writes one fully configured spacecraft definition and its requirements.
func (g *generator) satellite(id, plane, slot int) {
	g.stats.Satellites++
	g.decl(2, "part def Sat%d :> Spacecraft {", id)
	g.decl(3, "attribute :>> catalogId = %d;", 40000+id)
	g.decl(3, "attribute :>> plane = %d;", plane)
	g.decl(3, "attribute :>> slot = %d;", slot)
	var massTerms, powerTerms []string
	for _, s := range subsystems {
		massTerms = append(massTerms, s.name+".mass")
		powerTerms = append(powerTerms, s.name+".powerDraw")
		g.redefinePart(3, s.name)
		var subMass, subPower []string
		for j, c := range s.components {
			g.stats.Components++
			subMass = append(subMass, c.name+".mass")
			subPower = append(subPower, c.name+".powerDraw")
			g.redefinePart(4, c.name)
			g.decl(5, "attribute :>> mass = %d.%d [kg];", 2+(id+j)%40, (id*3+j)%10)
			g.decl(5, "attribute :>> powerDraw = %d.0 [W];", 5+(id*5+j*7)%50)
			g.decl(5, "attribute :>> serialNumber = \"%s-%05d-%d\";", strings.ToUpper(c.name), id, j)
			g.componentDetail(c.def, id, j)
			g.line(4, "}")
		}
		g.decl(4, "attribute :>> mass = %s;", strings.Join(subMass, " + "))
		g.decl(4, "attribute :>> powerDraw = %s;", strings.Join(subPower, " + "))
		g.line(3, "}")
	}
	g.decl(3, "attribute :>> dryMass = %s;", strings.Join(massTerms, " + "))
	g.decl(3, "attribute :>> totalPowerDraw = %s;", strings.Join(powerTerms, " + "))
	for _, s := range subsystems {
		if s.name == "eps" {
			continue
		}
		g.stats.Connections += 2
		g.decl(3, "interface powerTo%s : PowerFeed connect eps.supply to %s.power {", capitalize(s.name), s.name)
		g.decl(4, "attribute :>> busVoltage = 28 [V];")
		g.line(3, "}")
	}
	busIndex := 1
	for _, s := range subsystems {
		if s.name == "cdh" {
			continue
		}
		g.decl(3, "interface busTo%s : DataBusLink connect cdh.obc.bus%d to %s.bus;", capitalize(s.name), busIndex, s.name)
		busIndex++
	}
	g.stats.Connections++
	g.decl(3, "interface payloadToCdh : DataFeed connect payload.dataOut to cdh.dataIn;")
	g.decl(3, "constraint massMargin { dryMass <= %d [kg] }", 500+id%100)
	g.line(2, "}")
	g.decl(2, "part sat%dConfig : Sat%d;", id, id)
	g.stats.Requirements += 3
	g.decl(2, "requirement sat%dMass : MassBudget { subject :>> sc = sat%dConfig; attribute :>> limit = %d [kg]; }", id, id, 900+id%100)
	g.stats.Elements += 2
	g.decl(2, "satisfy sat%dMass by sat%dConfig;", id, id)
	g.decl(2, "requirement sat%dPower : PowerBudget { subject :>> sc = sat%dConfig; }", id, id)
	g.stats.Elements++
	g.decl(2, "satisfy sat%dPower by sat%dConfig;", id, id)
	g.decl(2, "requirement sat%dCrosslink : CrosslinkCapacity { subject :>> sc = sat%dConfig; attribute :>> minimumRate = %d.0; }", id, id, 50+id%50)
	g.stats.Elements += 2
	g.decl(2, "satisfy sat%dCrosslink by sat%dConfig;", id, id)
	g.line(0, "")
}

// componentDetail writes the as-built values of the attributes a component kind adds.
func (g *generator) componentDetail(def string, id, j int) {
	switch def {
	case "SolarArray":
		g.decl(5, "attribute :>> area = %d.%d ['m²'];", 4+id%6, id%10)
		g.decl(5, "attribute :>> generated = %d.0 [W];", 1200+(id*13)%700)
	case "Battery":
		g.decl(5, "attribute :>> capacity = %d.0 [J];", 3600000+(id*17)%7200000)
		g.decl(5, "attribute :>> depthOfDischarge = 0.%d;", 2+id%5)
	case "PowerConditioner":
		g.decl(5, "attribute :>> efficiency = 0.9%d;", id%10)
	case "StarTracker":
		g.decl(5, "attribute :>> accuracy = %d.0 [arcsec];", 1+id%5)
		g.decl(5, "attribute :>> updateRate = %d.0 [Hz];", 2+id%8)
	case "InertialMeasurementUnit":
		g.decl(5, "attribute :>> driftRate = 0.0%d;", 1+id%9)
	case "ReactionWheel":
		g.decl(5, "attribute :>> maxTorque = 0.%d ['N⋅m'];", 1+(id+j)%9)
		g.decl(5, "attribute :>> momentumCapacity = %d.0;", 10+(id+j)%40)
	case "OnboardComputer":
		g.decl(5, "attribute :>> clockRate = %d.0 [Hz];", 200000000+(id*11)%600000000)
		g.decl(5, "attribute :>> memoryBytes = %d;", (1+id%8)*1073741824)
	case "MassMemory":
		g.decl(5, "attribute :>> capacityBytes = %d;", (16+id%48)*1073741824)
	case "Transponder":
		g.decl(5, "attribute :>> frequency = %d.0 [Hz];", 8000000000+(id*7)%400000000)
		g.decl(5, "attribute :>> transmitPower = %d.0 [W];", 10+id%40)
	case "CrosslinkTerminal":
		g.decl(5, "attribute :>> wavelength = 1550 [nm];")
		g.dataRate(5, 100+(id*3)%400)
	case "Antenna":
		g.decl(5, "attribute :>> gain = %d.%d;", 20+id%20, id%10)
		g.decl(5, "attribute :>> diameter = 0.%d [m];", 3+id%6)
	case "PropellantTank":
		g.decl(5, "attribute :>> propellantMass = %d.0 [kg];", 30+(id*5)%100)
		g.decl(5, "attribute :>> volume = 0.%d ['m³'];", 1+id%5)
	case "Thruster":
		g.decl(5, "attribute :>> thrust = %d.0 [mN];", 20+id%200)
		g.decl(5, "attribute :>> specificImpulse = %d.0 [s];", 1200+(id*19)%800)
	case "Radiator":
		g.decl(5, "attribute :>> area = 1.%d ['m²'];", id%10)
		g.decl(5, "attribute :>> emissivity = 0.8%d;", id%10)
	case "Heater":
		g.decl(5, "attribute :>> setpoint = %d.0 [K];", 283+id%20)
	case "ImagingSensor":
		g.decl(5, "attribute :>> groundSampleDistance = 0.%d [m];", 3+id%7)
		g.decl(5, "attribute :>> swath = %d.0 [km];", 10+id%30)
	case "PayloadProcessor":
		g.decl(5, "attribute :>> throughput = %d.0;", 100+(id*23)%900)
	}
}

// groundStation writes one ground station definition with its as-built values.
func (g *generator) groundStation(k int) {
	g.stats.GroundStations++
	g.decl(2, "part def Station%d :> GroundStation {", k)
	g.decl(3, "attribute :>> stationId = %d;", k)
	g.decl(3, "attribute :>> latitude = %d.%d;", -60+(k*37)%120, k%10)
	g.decl(3, "attribute :>> longitude = %d.%d;", -180+(k*53)%360, k%10)
	for j, c := range stationComponents {
		g.stats.Components++
		g.redefinePart(3, c.name)
		g.decl(4, "attribute :>> mass = %d.0 [kg];", 50+(k*7+j*11)%900)
		g.decl(4, "attribute :>> powerDraw = %d.0 [W];", 100+(k*13+j*17)%2000)
		g.decl(4, "attribute :>> serialNumber = \"GS-%s-%03d\";", strings.ToUpper(c.name), k)
		g.componentDetail(c.def, k, j)
		g.line(3, "}")
	}
	g.line(2, "}")
	g.line(0, "")
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
