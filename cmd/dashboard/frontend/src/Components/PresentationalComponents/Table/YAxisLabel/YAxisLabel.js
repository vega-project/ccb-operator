import React, { Fragment } from 'react';
import PropTypes from 'prop-types';

const YAxisLabel = ({ label, index, stepper }) => (
    <Fragment>
    {
      index === 0 && (
        <td className="yaxis-label label" rowSpan={stepper.length}>
          <span className="vertical">{label}</span>
        </td>
      )
    }
    <td className="yaxis-label">
      { index%4 === 0 && <span className="label">{stepper[index].toFixed(1)}</span> } 
    </td>
    {
      index === 0 && (
        <td rowSpan={stepper.length} className="yaxis-line"/>
      )
    }
    </Fragment>
);

YAxisLabel.propTypes = {
    index: PropTypes.number.isRequired,
    label: PropTypes.string.isRequired,
    stepper: PropTypes.array.isRequired,
};

export default YAxisLabel;
