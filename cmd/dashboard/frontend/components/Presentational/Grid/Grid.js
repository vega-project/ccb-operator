import React, { Fragment } from 'react';
import classNames from 'classnames';
import PropTypes from 'prop-types';
import XaxisLabel from './Xaxis';
import YaxisLabel from './Yaxis';
import { Tooltip } from '@patternfly/react-core';
import { getCalculation, concatSpec } from '../../../utils/helper'

const Grid = ({ xaxis, yaxis, columns, rows, data, selected, setSelected }) => {

    const handleSelectCalc = (logG, teff) => {
        setSelected(concatSpec(logG, teff));
    };

    const getComputedStyle = (logG, teff) => {
        let calc = getCalculation(data, logG, teff)
        let phase = calc && calc.phase.toLowerCase();
        return classNames(
            'cell',
            phase,
            {
                active: !!calc,
                selected: selected === concatSpec(logG, teff)
            }
        );
    };
    return (
        <Fragment>
            <table className="table">
                <tbody>
                    {columns.map((column, columnIndex) => (
                        <tr key={columnIndex}>
                            <YaxisLabel
                                index={columnIndex}
                                label={yaxis.label}
                                stepper={columns}
                            />

                            {rows.map((row, rowIndex) => (
                                <td
                                    onClick={() => handleSelectCalc(column, row)}
                                    key={rowIndex}
                                    className={getComputedStyle(column, row)}
                                > 
                                    <Tooltip
                                        entryDelay={300}
                                        content={
                                            <Fragment>
                                                <div>LogG: {column}dex</div>
                                                <div>Teff: {row}K</div>
                                            </Fragment>
                                        }
                                    >
                                        <span className="dot">•</span>
                                    </Tooltip>
                                </td>
                            ))}
                        </tr>
                    ))}
                </tbody>
                <XaxisLabel stepper={rows} label={xaxis.label} />
            </table>
        </Fragment>
    );
};

Grid.propTypes = {
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

export default Grid;