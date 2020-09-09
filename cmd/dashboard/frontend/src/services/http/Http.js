import Client from './Client';

class Http extends Client {
    constructor() {
        super();
    }

    /**
     * Performing a GET request
     *
     * @param {String} url
     * @param {Object} params
     */
    get(url, params) {
        return super.request('GET', url, params);
    }

    /**
     * Performing a POST request
     *
     * @param {String} url
     * @param {Object} data
     */
    post(url, data) {
        return super.request('POST', url, data);
    }
}

export default new Http();
