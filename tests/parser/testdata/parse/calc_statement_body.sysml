package CalcStatements {
    calc def Factorial {
        in n : Integer;
        attribute acc : Integer = 1;
        attribute i : Integer = 1;
        while i <= n {
            assign acc := acc * i;
            assign i := i + 1;
        }
        return : Integer = acc;
    }

    calc def FirstOver {
        in xs : Integer[*];
        in limit : Integer;
        for x in xs {
            if x > limit {
                return : Integer = x;
            }
        }
        return : Integer = -1;
    }

    calc def Classify {
        in n : Integer;
        if n < 0 {
            return : String = "negative";
        } else {
            if n == 0 {
                return : String = "zero";
            }
            return : String = "positive";
        }
    }

    calc def Countdown {
        in n : Integer;
        attribute i : Integer = n;
        loop {
            assign i := i - 1;
        } until i <= 0;
        return : Integer = i;
    }
}
