import Http from './http/Http';

class Calculations {
    constructor() {
        // #TODO fix me, dynamic
        this.host = 'http://localhost:8080';
    }

    /**
     * Gets all the calculations
     *
     * @return {Object}
     */
    all() {
        const url = `${this.host}/calculations`;

        return Http.get(url);
    }

    /**
     * Gets the calculations for the given name
     *
     * @param {String} name - ex. calc-psnh7dp2js0tfl7w
     * @return {Object}
     */
    calculation(name) {
        if (typeof name === 'undefined') {
            throw new Error('calculation name is undefined');
        }

        const url = `${this.host}/calculation/${name}`;

        return Http.get(url);
    }

    delete(name) {
        if (typeof name === 'undefined') {
            throw new Error('calculation name is undefined');
        }

        const url = `${this.host}/calculations/delete/${name}`;
        return Http.delete(url);
    }

}

export default new Calculations();
