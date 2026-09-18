// Census: validateViewpointUsageType
// Expect: error: A viewpoint must be typed by one viewpoint definition.
package P { viewpoint def V1; viewpoint def V2; viewpoint v : V1, V2; }
