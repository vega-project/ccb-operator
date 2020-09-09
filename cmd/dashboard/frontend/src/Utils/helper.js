/** Generates and array from the range
 *
 * @param {Number} start
 * @param {Number} total
 * @param {Number} step stepper which increases the array
 *
 * @returns Array
 */
export const range = ({ start, total, step }, fixed=true) => Array.from({ length: total + 1 }, (v, index) => {
    let value = start + index * step;
    if (fixed) {
        return value.toFixed(1)
    }
    return value;
});

/**
 *  Groups array by spec
 *  @param {Array} array
 *   @returns object
 */
export const groupArrayBySpec = (array) => (
    array.reduce((acc, calc) => { 
        let { spec } = calc; 
        let { logG, teff } = spec;
        let fixedLogG = logG.toFixed(1);
        acc[fixedLogG] = { 
            ...acc[fixedLogG], 
            [teff]: { ...calc } 
        };
        return acc
    }, {})
)

export const getCalculation = (data, logG, teff) => data[logG] && data[logG][teff] ? data[logG][teff] : undefined;
export const concatSpec = (logG, teff) => ''.concat(logG, '-', teff);
export const reverseSpec = (calc) => calc.split('-');