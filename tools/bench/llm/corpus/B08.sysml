package Login {
    private import ScalarValues::*;

    action def ReadCredentials {
        out username : String;
        out password : String;
    }

    action def VerifyCredentials {
        in username : String;
        in password : String;
        out valid : Boolean;
    }

    action def GrantAccess;
    action def DenyAccess;

    action def Authenticate {
        out granted : Boolean;

        action read : ReadCredentials;
        then action validation : VerifyCredentials;
        flow read.username to validation.username;
        flow read.password to validation.password;

        then decide;
            if validation.valid then grant;
            else deny;
        action grant : GrantAccess;
        action deny : DenyAccess;
        then merge finish;
        then done;
    }
}
