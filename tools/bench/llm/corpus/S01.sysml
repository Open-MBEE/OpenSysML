package Bicycle {
    part def Frame;
    part def Wheel;
    part def Saddle;

    part def Bicycle {
        part mainFrame : Frame;
        part frontWheel : Wheel;
        part rearWheel : Wheel;
        part saddle : Saddle;
    }
}
