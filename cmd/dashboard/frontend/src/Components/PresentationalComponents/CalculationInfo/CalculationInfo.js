import React, { Fragment, useState, useEffect } from 'react';
import PropTypes from 'prop-types';
import { Card } from '@patternfly/react-core/dist/esm/components/Card';
import { CardBody } from '@patternfly/react-core/dist/esm/components/Card/CardBody';
import { CardHeader } from '@patternfly/react-core/dist/esm/components/Card/CardHeader';
import { Flex, FlexItem } from '@patternfly/react-core';
import Calculations from '../../../../services/Calculations';
import {
    TextContent,
    Text,
    TextVariants
} from '@patternfly/react-core';
import { Stack, StackItem } from '@patternfly/react-core';

const CalculationInfo = ({ calculation }) => {
    const [data, setData] = useState(undefined);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        Calculations.calculation(calculation)
        .then(response => {
            setData(response.data);
        })
        .catch(error => error)
        .finally(() => setLoading(false));
    }, [calculation]);
    return (
        <Fragment>
            <Flex>
                <FlexItem>
                    {data && <Card>
                        <CardHeader>Model </CardHeader>
                        <CardBody>
                            <TextContent>
                                <Stack>
                                    <StackItem><Text component={TextVariants.p}>ATLAS</Text></StackItem>
                                    <StackItem >Teff = {data.spec.Teff} K</StackItem>
                                    <StackItem >LoGg = {data.spec.LogG} dex</StackItem>
                                    <StackItem >Metalicity = 0 dex</StackItem>
                                    <StackItem >Vmicro = 0 km/s</StackItem>
                                    <StackItem>status = {data.spec.status}</StackItem>
                                </Stack>

                            </TextContent>
                        </CardBody>
                    </Card>
                    }
                </FlexItem>

                <FlexItem>
                    <Card isFlat>
                        <CardHeader>Surface distribution </CardHeader>
                        <CardBody>Body</CardBody>
                    </Card>
                </FlexItem>
            </Flex>
        </Fragment>
    );
};

CalculationInfo.propTypes = {
    calculation: PropTypes.string.isRequired
};

export default CalculationInfo;
