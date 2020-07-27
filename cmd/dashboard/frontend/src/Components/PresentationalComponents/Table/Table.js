import React, { Fragment, useState } from 'react';
import PropTypes from 'prop-types';
import classNames from 'classnames';
import YAxisLabel from './YAxisLabel/YAxisLabel';
import XAxisLabel from './XAxisLabel/XAxisLabel';
import TableToolBar from '../TableToolbar/TableToolBar';
import CalculationInfo from '../CalculationInfo/CalculationInfo';

const Table = ({ xaxis, yaxis, columns, rows, data }) => {
    const [selected, setSelected] = useState();
    const [calcName, setCalcName] = useState();
    const calculations = [];

    const handleSelectRow = (event, column, row, columnIndex, rowIndex) => {
        let calcName = event.target.parentElement.getAttribute('data-calc-name');
        setCalcName(calcName);
        setSelected(`${column}-${row}`);
    };

    const filterCalc = (logG, teff, index) => {
        let filtered = data && data.items.filter(({ spec }) => spec.Teff === teff && spec.LogG === logG);
        calculations[index] = filtered;
        return filtered;
    };

    const getCalcName = (index) => {
        if (calculations[index] && calculations[index].length > 0) {
            let [calc] = calculations[index];
            return calc.metadata.name;
        }

        return null;
    };

    const getComputedStyle = (logG, teff, index) => {
        let phase = '';
        let filtered = filterCalc(logG, teff, index);

        if (filtered && filtered.length) {
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
                                onClick={(event) => handleSelectRow(event, column, row, columnIndex, rowIndex)}
                                className={getComputedStyle(column, row, rowIndex)}
                                data-calc-name={getCalcName(rowIndex)}

                            >
                                <span className="dot">•</span>
                            </td>
                        ))}
                    </tr>
                ))}
                <XAxisLabel stepper={rows} label={xaxis.label} />
            </table>
            { selected &&  <TableToolBar /> }
            { selected &&  <CalculationInfo calculation={calcName}/> }

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
