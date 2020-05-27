import React, { Fragment, useState } from 'react';
import PropTypes from 'prop-types';
import classNames from 'classnames';
import YAxisLabel from './YAxisLabel/YAxisLabel';
import XAxisLabel from './XAxisLabel/XAxisLabel';

const Table = ({ xaxis, yaxis, columns, rows, data }) => {
    const [selected, setSelected] = useState();

    const handleSelectRow = (evn, column, row, columnIndex, rowIndex) => {
        setSelected(`${column}-${row}`);
    };

    const getComputedStyle = (logG, teff) => {
        let phase = '';
        let filtered = data && data.items.filter(({ spec }) => spec.Teff === teff && spec.LogG === logG);

        if (filtered.length) {
            let [calc] = filtered;
            phase = calc.phase.toLowerCase();
        }

        return classNames({
            cell: true,
            selected: selected === `${logG}-${teff}`,
            active: phase === 'created',
            processing: phase === 'processing'
        });
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
                                className={getComputedStyle(column, row)}
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
    rows: PropTypes.array.isRequired,
    data: PropTypes.object.isRequired
};

export default Table;
