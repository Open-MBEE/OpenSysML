#!/usr/bin/env bash
# Single source of the fUML reference-implementation pin (ModelDriven, AFL 3.0):
# release tag, commit, the sha256 of each model and jar, and the jar's Maven runtime
# dependencies. Change tag, commit and checksums together; see docs/project/fuml-referee.md.
FUML_RI_TAG="${FUML_RI_TAG:-v1.5.0a}"
FUML_RI_COMMIT="${FUML_RI_COMMIT:-45e506336d4cd56965d4ad3b684149245f899f3a}"
FUML_RI_REPO="${FUML_RI_REPO:-https://github.com/ModelDriven/fUML-Reference-Implementation}"
FUML_RI_RAW="${FUML_RI_RAW:-https://raw.githubusercontent.com/ModelDriven/fUML-Reference-Implementation/$FUML_RI_COMMIT/org.modeldriven.fuml}"

FUML_TESTS_FILE="fUML-Tests.uml"
FUML_TESTS_URL="${FUML_TESTS_URL:-$FUML_RI_RAW/src/test/resources/uml/fUML-Tests.uml}"
FUML_TESTS_SHA256="${FUML_TESTS_SHA256:-c7d54bf2b427f65b17e5fbd817b24cbe4a8cad2b7785233ef19f72a09b77e0f0}"

FUML_EXCEPTION_TESTS_FILE="fUML-Exception-Tests.uml"
FUML_EXCEPTION_TESTS_URL="${FUML_EXCEPTION_TESTS_URL:-$FUML_RI_RAW/src/test/resources/uml/fUML-Exception-Tests.uml}"
FUML_EXCEPTION_TESTS_SHA256="${FUML_EXCEPTION_TESTS_SHA256:-4e8315803ffafd1b475847638a7a5e13716b7aea9d7098044cc85b984932d74b}"

FUML_LIBRARY_FILE="fUML_Library.xmi"
FUML_LIBRARY_URL="${FUML_LIBRARY_URL:-$FUML_RI_RAW/src/test/resources/fUML_Library.xmi}"
FUML_LIBRARY_SHA256="${FUML_LIBRARY_SHA256:-7e8bae518398fd44d788e9c1145c7a0754bde5935c1e6127fec9878e8e2da8b7}"

FUML_JAR_FILE="fuml-1.5.0a.jar"
FUML_JAR_URL="${FUML_JAR_URL:-$FUML_RI_RAW/dist/fuml-1.5.0a.jar}"
FUML_JAR_SHA256="${FUML_JAR_SHA256:-4e78a194bdd15ed4ee30b26828a91d3b2bd32e57cd397667e32b21eb4de1f06e}"

# The namespace URIs the tests are loaded under; the JUnit suite uses the same.
FUML_TESTS_URI="http://org.modeldriven.fuml/test/uml/papyrus/fUML-Tests"
FUML_EXCEPTION_TESTS_URI="http://org.modeldriven.fuml/test/uml/papyrus/fUML-Exception-Tests"

FUML_MAVEN_REPO="${FUML_MAVEN_REPO:-https://repo1.maven.org/maven2}"
# One "repository-path sha256" pair per line, in classpath order.
FUML_DEPS="\
net/java/dev/stax-utils/stax-utils/20040917/stax-utils-20040917.jar e281bd1a1f046f60688e272c32afa83005be7ee84513688c74dc007a440c18e7
com/sun/xml/bind/jaxb-impl/3.0.0/jaxb-impl-3.0.0.jar 6d5a3cddb6948b6adc6707b121d627c3b50bc23334e072e8ddd8e82b894f384e
com/sun/xml/stream/sjsxp/1.0.1/sjsxp-1.0.1.jar 7d2e05b5e2111b52d814371a16b5aec1bf7b7d7cb315f5d34be2c475d9c3d0ac
commons-logging/commons-logging/1.1.1/commons-logging-1.1.1.jar ce6f913cad1f0db3aad70186d65c5bc7ffcc9a99e3fe8e0b137312819f7c362f
xerces/xercesImpl/2.12.2/xercesImpl-2.12.2.jar 6fc991829af1708d15aea50c66f0beadcd2cfeb6968e0b2f55c1b0909883fe16
jakarta/xml/bind/jakarta.xml.bind-api/3.0.0/jakarta.xml.bind-api-3.0.0.jar 0037ab1eba33a969b36119c61a9f28313460a85eea9b800470a70cb2227d35f4
commons-collections/commons-collections/3.2.2/commons-collections-3.2.2.jar eeeae917917144a68a741d4c0dff66aa5c5c5fd85593ff217bced3fc8ca783b8
xalan/serializer/2.7.2/serializer-2.7.2.jar e8f5b4340d3b12a0cfa44ac2db4be4e0639e479ae847df04c4ed8b521734bb4a
log4j/log4j/1.2.8/log4j-1.2.8.jar c316595a68f7bc74ee0931e0c4435481cdeddc91c95d2cb78eada107c5b01a65
com/sun/xml/bind/jaxb-core/3.0.0/jaxb-core-3.0.0.jar b1e4b8137505333ed59b08e748c46c3894795fa1d524d042354609fd9ca6754f
xalan/xalan/2.7.2/xalan-2.7.2.jar a44bd80e82cb0f4cfac0dac8575746223802514e3cec9dc75235bc0de646af14
xml-apis/xml-apis/1.4.01/xml-apis-1.4.01.jar a840968176645684bb01aed376e067ab39614885f9eee44abe35a5f20ebe7fad
javax/xml/stream/stax-api/1.0/stax-api-1.0.jar 6d51230129cf695d21928798e352960d450397ee3c830b4e12948f3e3cfab90d
com/sun/activation/jakarta.activation/2.0.0/jakarta.activation-2.0.0.jar db9c7e30f4d61ec33fd47942c9b7cf6e094025e7d5d8e20db73fce5a912a4366
commons-lang/commons-lang/2.1/commons-lang-2.1.jar 2ded7343dc8e57decd5e6302337139be020fdd885a2935925e8d575975e480b9"

# fuml_pin is the stamp a fetched destination records; a change to any part of it re-fetches.
fuml_pin() {
	printf '%s %s %s %s %s %s %s' "$FUML_RI_TAG" "$FUML_RI_COMMIT" \
		"$FUML_TESTS_SHA256" "$FUML_EXCEPTION_TESTS_SHA256" "$FUML_LIBRARY_SHA256" "$FUML_JAR_SHA256" \
		"$(printf '%s' "$FUML_DEPS" | sha256sum | cut -d' ' -f1)"
	return 0
}

# fuml_dep_files lists the dependency jar names in classpath order.
fuml_dep_files() {
	printf '%s\n' "$FUML_DEPS" | while read -r path _; do
		printf '%s\n' "${path##*/}"
	done
	return 0
}
