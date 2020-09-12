/*eslint-disable no-useless-constructor */
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
    post(url, params, data) {
        return super.request('POST', url, params,  data);
    }

    /**
     * Performing a DELETE request
     *
     * @param {String} url
     * @param {Object} data
     */
    delete(url, data) {
        return super.request('DELETE', url, data);
    }
}

export default new Http();
