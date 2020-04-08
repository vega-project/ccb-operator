/** Generates and array from the range
 *
 * @param {Number} start
 * @param {Number} total
 * @param {Number} step stepper which increases the array
 *
 * @returns Array
 */
export const range = ({ start, total, step }) => Array.from({ length: total + 1 }, (v, index) => start + index * step);
