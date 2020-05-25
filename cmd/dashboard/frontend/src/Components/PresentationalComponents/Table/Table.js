import React, { Fragment, useState } from 'react';
import PropTypes from 'prop-types';
import YAxisLabel from './YAxisLabel/YAxisLabel';
import XAxisLabel from './XAxisLabel/XAxisLabel';

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
                        <YAxisLabel
                            index={columnIndex}
                            label={yaxis.label}
                            stepper={columns}
                        />

                        {rows.map((row, rowIndex) => (
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
                <XAxisLabel stepper={rows} label={xaxis.label} />
            </table>
            <p>You selected: {selected}</p>
        </Fragment>
    );
};

Table.propTypes = {
    xaxis: PropTypes.shape({
        label: PropTypes.string,
        stepper: PropTypes.number
    }).isRequired,
    yaxis: PropTypes.shape({
        label: PropTypes.string,
        stepper: PropTypes.number
    }).isRequired,
    columns: PropTypes.array.isRequired,
    rows: PropTypes.array.isRequired
};

export default Table;
