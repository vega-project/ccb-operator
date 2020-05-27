import React, { Fragment } from 'react';
import PropTypes from 'prop-types';

const XAxisLabel = ({ label, stepper }) => (
    <Fragment>
        <tr>
            <td colSpan="3" />
            <td colSpan={stepper.length} className="xaxis-line" />
        </tr>
        <tr>
            <td colSpan="3" />
            <td colSpan={stepper.length}/>
        </tr>
        <tr>
            <td colSpan="3" />
            {stepper.map((row, index) => {
                return index % 4 === 0 ? <td rowSpan="3" className="label label-y">{row}</td> : <td rowSpan="3"/>;
            })}
        </tr>
        <tr></tr>
        <tr></tr>
        <tr className="x-axis-label label">
            <td colSpan="3" />
            <td rowSpan="2" colSpan={stepper.length}>
                <p>{label}</p>
            </td>
        </tr>
    </Fragment>
);

XAxisLabel.propTypes = {
    label: PropTypes.string.isRequired,
    stepper: PropTypes.array.isRequired
};

export default XAxisLabel;
