# Rover Requirement Satisfaction

Which part of the rover satisfies each requirement.

<!-- caption -->
*Requirements and their satisfiers*

| shortName | documentation | satisfiedBy | satisfied |
| --- | --- | --- | --- |
| RV-1 | The rover shall travel at least 20 km on one charge. | RoverBasic::rover::battery | true |
| RV-2 | The rover shall carry a 15 kg science payload. | RoverBasic::rover::chassis | true |
| RV-3 | The rover shall accept commands from the lander at 2 kbps. |  | false |

Requirements no part satisfies:

- RV-3
