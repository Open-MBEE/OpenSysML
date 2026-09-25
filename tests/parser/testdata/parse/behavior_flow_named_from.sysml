action def Powertrain {
    action generateTorque { out engineTorque : Real; }
    action amplifyTorque { in engineTorque : Real; }
    flow generateToAmplify from generateTorque.engineTorque to amplifyTorque.engineTorque;
    flow from generateTorque.engineTorque to amplifyTorque.engineTorque;
}
