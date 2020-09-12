import { 
    range,
    groupArrayBySpec,
    getCalculation,
    concatSpec,
    reverseSpec
 } from "./helper";

describe("helper.js", () => {
    describe("[range]", () => {
        it("Should return fixed range", () => {
            let object =  {
                start: 1,
                end: 5,
                total: 4,
                step: 1.400
            }
            const expected = [ "1.0", "2.4", "3.8", "5.2", "6.6" ]
            const result = range(object)
            expect(expected).toStrictEqual(result);
    
        })
    
        it("Should return range", () => {
            let object =  {
                start: 1,
                end: 5,
                total: 4,
                step: 1.400
            }
            const expected = [ 1, 2.4, 3.8, 5.199999999999999, 6.6 ]
            const result = range(object, false)
            expect(expected).toStrictEqual(result);
    
        })
    })

    describe("[groupArrayBySpec]", () => {
        it("Should group data by spec", () => {
            const data = [
                {
                    "kind":"Calculation",
                    "apiVersion":"vega.io/v1",
                    "metadata": {"name":"calc-ptq39yj8cmbbwlcf","creationTimestamp":null},
                    "spec": {
                        "steps":[
                            {
                                "command":"atlas12_ada",
                                "args":["s"]
                            },
                            {
                                "command":"atlas12_ada",
                                "args":["r"]
                            },
                            {
                                "command":"synspec49",
                                "args":["\u003c","input_tlusty_fortfive"]
                            }
                        ],
                        "teff":10003,
                        "logG":4
                    },
                    "dbkey":"vz.star:teff_10003","assign":"",
                    "status":
                        {
                            "startTime":"2020-09-12T21:00:50Z"
                        },
                        "phase":"Created"
                },
                {
                    "kind":"Calculation",
                    "apiVersion":"vega.io/v1",
                    "metadata": {"name":"calc-ptq39yj8cmbbwlcf","creationTimestamp":null},
                    "spec": {
                        "steps":[
                            {
                                "command":"atlas12_ada",
                                "args":["s"]
                            },
                            {
                                "command":"atlas12_ada",
                                "args":["r"]
                            },
                            {
                                "command":"synspec49",
                                "args":["\u003c","input_tlusty_fortfive"]
                            }
                        ],
                        "teff":10005,
                        "logG":4
                    },
                    "dbkey":"vz.star:teff_10003","assign":"",
                    "status":
                        {
                            "startTime":"2020-09-12T21:00:50Z"
                        },
                    "phase":"Created"
                }

            ]
            const expected = {
                "4.0": {
                  "10003": {
                    "kind": "Calculation",
                    "apiVersion": "vega.io/v1",
                    "metadata": {"name":"calc-ptq39yj8cmbbwlcf","creationTimestamp":null},
                    "spec": {
                        "steps":[
                            {
                                "command":"atlas12_ada",
                                "args":["s"]
                            },
                            {
                                "command":"atlas12_ada",
                                "args":["r"]
                            },
                            {
                                "command":"synspec49",
                                "args":["\u003c","input_tlusty_fortfive"]
                            }
                        ],
                        "teff":10003,
                        "logG":4
                    },
                    "dbkey": "vz.star:teff_10003",
                    "assign": "",
                    "status": {"startTime":"2020-09-12T21:00:50Z"},
                    "phase": "Created"
                  },
                  "10005": {
                    "kind": "Calculation",
                    "apiVersion": "vega.io/v1",
                    "metadata": {"name":"calc-ptq39yj8cmbbwlcf","creationTimestamp":null},
                    "spec": {
                        "steps":[
                            {
                                "command":"atlas12_ada",
                                "args":["s"]
                            },
                            {
                                "command":"atlas12_ada",
                                "args":["r"]
                            },
                            {
                                "command":"synspec49",
                                "args":["\u003c","input_tlusty_fortfive"]
                            }
                        ],
                        "teff":10005,
                        "logG":4
                    },
                    "dbkey": "vz.star:teff_10003",
                    "assign": "",
                    "status": {"startTime":"2020-09-12T21:00:50Z"},
                    "phase": "Created"
                  },
                  
                }
              }
          
            const result = groupArrayBySpec(data)
            expect(expected).toStrictEqual(result);
        })

    })

    describe("[getCalculation]", () => {
        it("Should return calculation for the given teff/logG", () => {
            
            const data = {
                "4.0": {
                    "10003": {
                        "metadata": {"name":"calc-ptq39yj8cmbbwlcf","creationTimestamp":null},
                    }
                }
            };

            const calc = getCalculation(data, "4.0", "10003");
            expect(calc.metadata.name).toBe("calc-ptq39yj8cmbbwlcf")

        })

        it("Should return undefined calculation", () => {
            
            const data = {
                "4.0": {
                    "10003": {
                        "metadata": {"name":"calc-ptq39yj8cmbbwlcf","creationTimestamp":null},
                    }
                }
            };

            let calc = getCalculation(data, "non-existent", "10003");
            expect(calc).toBe(undefined)

            calc = getCalculation(data, "4.0", "non-existent");
            expect(calc).toBe(undefined)

        })
    })

    describe("Spec helper", () => {
        it("Should concat given values with - seperator", () => {
            let spec = concatSpec(4, 10)
            expect(spec).toBe("4-10")
        })

        it("Should split values by -", () => {
            let spec = reverseSpec("4-10")
            expect(spec).toStrictEqual(["4", "10"])
        })
    })
   
});
