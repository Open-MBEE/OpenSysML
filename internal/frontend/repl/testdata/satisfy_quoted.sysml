package 'Landing Site' {
    part def 'Space Craft' {
        attribute 'vertical speed';
    }

    requirement def 'Touchdown Requirement' {
        subject craft : 'Space Craft';
        attribute 'max vertical speed';
        require constraint {
            craft.'vertical speed' <= 'max vertical speed'
        }
    }

    requirement 'the touchdown' : 'Touchdown Requirement' {
        attribute :>> 'max vertical speed' = 1.5;
    }

    part 'slow craft' : 'Space Craft' {
        attribute :>> 'vertical speed' = 1.2;
    }

    part 'fast craft' : 'Space Craft' {
        attribute :>> 'vertical speed' = 2.4;
    }

    part 'analysis context' {
        assert satisfy 'the touchdown' by 'slow craft';
        assert satisfy 'the touchdown' by 'fast craft';
        assert not satisfy 'the touchdown' by 'fast craft';
    }
}
