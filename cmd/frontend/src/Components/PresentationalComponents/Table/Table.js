import React, { Fragment, useState } from 'react';
import PropTypes from 'prop-types';

const YAxisLabel = ({ length, label }) => (
  <Fragment>
    <td className="y-axis-label label" rowSpan={length}>
      <p className="vertical">{label}</p>
    </td>
    <td rowSpan={length} className="yaxis-line" />
  </Fragment>
);
const XAxisLabel = ({ length, label }) => (
  <Fragment>
    <tr>
      <td colSpan="2" />
      <td colSpan={length} className="xaxis-line" />
    </tr>
    <tr className="x-axis-label label">
      <td colSpan="3" />
      <td colSpan={length}>
        <p>{label}</p>
      </td>
    </tr>
  </Fragment>
);

XAxisLabel.propTypes = {
  length: PropTypes.number.isRequired,
  label: PropTypes.string.isRequired
};

YAxisLabel.propTypes = {
  length: PropTypes.number.isRequired,
  label: PropTypes.string.isRequired
};
const Table = ({ xaxis, yaxis, columns, rows }) => {
  const [selectedIndex, setSelectedIndex] = useState([]);
  const [selected, setSelected] = useState();

  const handleSelectRow = (evn, column, row, columnIndex, rowIndex) => {
    // TODO  make indexes better, safer
    setSelectedIndex(`${columnIndex},${rowIndex}`);
    setSelected(`${column},${row}`);
  };

  return (
    <Fragment>
      <table className="table">
        {columns.map((column, columnIndex) => (
          <tr key={columnIndex}>
            {columnIndex === 0 && <YAxisLabel length={columns.length} label={yaxis} />}

            {rows.reverse().map((row, rowIndex) => (
              <td
                key={rowIndex}
                onClick={evt => handleSelectRow(evt, column, row, columnIndex, rowIndex)}
                className={`cell ${selectedIndex === `${columnIndex},${rowIndex}` ? 'selected' : ''} ${
                  rowIndex / columnIndex > 1 && rowIndex / columnIndex < 1.8 ? 'active' : 'disable'
                }`}
              >
                <span className="dot">•</span>
              </td>
            ))}
          </tr>
        ))}
        <XAxisLabel length={rows.length} label={xaxis} />
      </table>
      <p>You selected: {selected}</p>
    </Fragment>
  );
};

Table.propTypes = {
  xaxis: PropTypes.string.isRequired,
  yaxis: PropTypes.string.isRequired,
  columns: PropTypes.array.isRequired,
  rows: PropTypes.array.isRequired
};

export default Table;
