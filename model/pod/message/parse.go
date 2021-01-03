/*
 * Copyright (C) 2020 Nicolas SCHWARTZ
 *
 * This library is free software; you can redistribute it and/or
 * modify it under the terms of the GNU Lesser General Public
 * License as published by the Free Software Foundation; either
 * version 2 of the License, or (at your option) any later version.
 *
 * This library is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU General Public
 * License along with this library; if not, write to the Free Software
 * Foundation, Inc., 51 Franklin St, Fifth Floor, Boston, MA 02110-1301, USA
 */

package message

import "encoding/binary"
import "fmt"
import "bytes"
import "log"
import "reflect"

import "github.com/StarAurryon/lpedit-lib/model/pod"

type presetPedalPos struct {
    pid   uint32
    ptype uint8
}

func (m Message) getPedalBoardItemID() uint32 {
    return binary.LittleEndian.Uint32(m.data[12:16])
}

func (m ActiveChange) Parse(p *pod.Pod) (error, int, interface{}) {
    id := m.getPedalBoardItemID()
    pbi := p.GetCurrentPreset().GetItem(id)
    if pbi == nil {
        return fmt.Errorf("Item ID %d not found", id), ct.StatusWarning(), nil
    }
    var active bool
    if binary.LittleEndian.Uint32(m.data[16:]) > 0 {
        active = true
    } else {
        active = false
    }
    log.Printf("Active change on ID %d status %t\n", id, active)
    pbi.SetActive(active)
    return nil, ct.StatusActiveChange(), pbi
}

func (m *Message) parseParameterChange(paramFunc string, p *pod.Pod) (error, int, interface{}) {
    pid := m.getPedalBoardItemID()
    pbi := p.GetCurrentPreset().GetItem(pid)
    if pbi == nil {
        return fmt.Errorf("Item ID %d not found", pid), ct.StatusWarning(), nil
    }
    id := binary.LittleEndian.Uint32(m.data[20:24])

    var v [4]byte
    copy(v[:], m.data[24:])

    param := pbi.GetParam(id)
    if param == nil {
        return fmt.Errorf("Parameter ID %d not found", id), ct.StatusWarning(), nil
    }
    if err := reflect.ValueOf(param).MethodByName(paramFunc).Interface().(func([4]byte) error)(v); err != nil {
        log.Printf("TODO: Fix the parameter type on pod.%s, parameter %s, func %s: %s \n", pbi.GetName(), param.GetName(), paramFunc, err)
    }
    return nil, ct.StatusNone(), param
}

func (m ParameterChange) Parse(p *pod.Pod) (error, int, interface{}) {
    err, pt, obj := m.parseParameterChange("SetBinValueCurrent", p)
    if err != nil { return err, pt, obj }
    return err, ct.StatusParameterChange(), obj
}

func (m ParameterChangeMin) Parse(p *pod.Pod) (error, int, interface{}) {
    err, pt, obj := m.parseParameterChange("SetBinValueMin", p)
    if err != nil { return err, pt, obj }
    return err, ct.StatusParameterChangeMin(), obj
}

func (m ParameterChangeMax) Parse(p *pod.Pod) (error, int, interface{}) {
    err, pt, obj := m.parseParameterChange("SetBinValueMax", p)
    if err != nil { return err, pt, obj }
    return err, ct.StatusParameterChangeMax(), obj
}

func (m ParameterTempoChange) Parse(p *pod.Pod) (error, int, interface{}) {
    pid := m.getPedalBoardItemID()
    pbi := p.GetCurrentPreset().GetItem(pid)
    if pbi == nil {
        return fmt.Errorf("Item ID %d not found", pid), ct.StatusWarning(), nil
    }
    param := pbi.GetParam(0)
    if param == nil {
        return fmt.Errorf("Parameter ID 0 not found"), ct.StatusWarning(), nil
    }
    value := float32(binary.LittleEndian.Uint32(m.data[16:]))
    binValue := [4]byte{}
    buf := new(bytes.Buffer)
    binary.Write(buf, binary.LittleEndian, value)
    copy(binValue[:], buf.Bytes())
    param.SetBinValueCurrent(binValue)
    return nil, ct.StatusParameterChange(), param
}

func (m ParameterTempoChange2) Parse(p *pod.Pod) (error, int, interface{}) {
    pid := m.getPedalBoardItemID()
    pbi := p.GetCurrentPreset().GetItem(pid)
    if pbi == nil {
        return fmt.Errorf("Item ID %d not found", pid), ct.StatusWarning(), nil
    }
    param := pbi.GetParam(2)
    if param == nil {
        return fmt.Errorf("Parameter ID 2 not found"), ct.StatusWarning(), nil
    }
    value := float32(binary.LittleEndian.Uint32(m.data[16:]))
    binValue := [4]byte{}
    buf := new(bytes.Buffer)
    binary.Write(buf, binary.LittleEndian, value)
    copy(binValue[:], buf.Bytes())
    param.SetBinValueCurrent(binValue)
    return nil, ct.StatusParameterChange(), param
}

func (m PresetChange) Parse(p *pod.Pod) (error, int, interface{}) {
    p.SetCurrentPreset(m.data[8])
    return nil, ct.StatusPresetChange(), p.GetCurrentPreset()
}

func (m PresetChangeAlert) Parse(p *pod.Pod) (error, int, interface{}) {
    return nil, ct.StatusNone(), nil
}

func (m PresetLoad) Parse(p *pod.Pod) (error, int, interface{}) {
    preset := p.GetCurrentPreset()
    pbiOrder := []uint32{0,2,1,3,4,5,6,7,8,9,10,11}
    name := [16]byte{}
    copy(name[:], m.data[8:24])
    preset.SetName(name)

    const offset = 48
    var data [256]byte
    for i, id := range pbiOrder {
        start := offset + (i * 256)
        end := start + 256
        copy(data[:], m.data[start:end])
        m.parsePedalBoardItem(preset, data, id)
    }
    m.parseDT(preset, m.data)
    m.parseCabs(preset, m.data)
    m.parseSetup(preset, m.data)
    return nil, ct.StatusPresetLoad(), preset
}

func (m PresetLoad) parseCabs(p *pod.Preset, data []byte) {
    cabs := []*pod.Cab {p.GetCab(0), p.GetCab(1)}
    offset := [2][2]int{[2]int{3412, 4096}, [2]int{3420, 4097}}
    parametersID := [2]uint32{pod.CabERID, pod.CabMicID}
    parametersSize := [2]int{4, 1}
    for i, cab := range cabs {
        if cab == nil {
            log.Printf("Can't find Cab ID %d\n", i)
            continue
        }
        for j, pType := range parametersID {
            param := cab.GetParam(pType)
            if param == nil {
                log.Printf("Can't find Cab ID %d, parameter %d\n", i, pType)
                continue
            }
            value := [4]byte{}
            copy(value[:], data[offset[i][j]:offset[i][j]+parametersSize[j]])
            if err := param.SetBinValueCurrent(value); err != nil {
                log.Printf("Can't set value Cab ID %d, parameter %d: %s\n", i, pType, err)
            }
        }
    }
}

func (m PresetLoad) parseDT(p *pod.Preset, data []byte) {
    dts := []*pod.DT{p.GetDT(0), p.GetDT(1)}
    offset := [2][3]int{[3]int{3124,3125,3126}, [3]int{3132, 3133, 3134}}
    for i, dt := range dts {
        if dt == nil {
            log.Printf("Can't find DT ID %d\n", i)
        } else {
            if err := dt.SetBinTopology(data[offset[i][0]]); err != nil {
                log.Printf("Error while setting DT ID %d Topology: %s\n", i, err)
            }
            if err := dt.SetBinClass(data[offset[i][1]]); err != nil {
                log.Printf("Error while setting DT ID %d Class: %s\n", i, err)
            }
            if err := dt.SetBinMode(data[offset[i][2]]); err != nil {
                log.Printf("Error while setting DT ID %d Mode: %s\n", i, err)
            }
        }
    }
}

func (m PresetLoad) parseSetup(p *pod.Preset, data []byte) {
    params := []uint32{pod.PresetGuitarInZ, pod.PresetInput1Source,
        pod.PresetInput2Source}
    offset := []int{3546, 4102, 4103}
    for i, pType := range params {
        param := p.GetParam(pType)
        if param == nil {
            log.Printf("Can't find PedalBoard Parameter ID %d\n", pType)
            continue
        }
        value := [4]byte{}
        value[0] = data[offset[i]]
        if err := param.SetBinValueCurrent(value); err != nil {
            log.Printf("Error while setting PedalBoard Parameter ID %d: %s\n", pType, err)
        }
    }
}

func (m PresetLoad) parsePedalBoardItem(p *pod.Preset, data [256]byte, pbiID uint32) {
    pbi := p.GetItem(pbiID)

    pbiType := binary.LittleEndian.Uint32(data[0:4])
    pbi.SetType(pbiType)

    pos := binary.LittleEndian.Uint16(data[4:6])
    posType := uint8(data[6])
    pbi.SetPosWithoutCheck(pos, posType)

    active := false
    if data[8] == 1 { active = true }
    pbi.SetActive(active)

    tempos := []uint8{data[9], data[10]}

    const offset = 16
    var paramData [20]byte
    switch pbi.(type) {
    case *pod.Cab:
        for i, pType := range []uint32{pod.CabLowCutID, pod.CabResLevelID,
            pod.CabThumpID, pod.CabDecayID} {
            start := offset + (i * 20)
            end := start + 20
            copy(paramData[:], data[start:end])
            m.parseParameterCab(pbi, paramData, pType)
        }
    default:
        for i := uint16(0); i < pbi.GetParamLen(); i++ {
            start := offset + (i * 20)
            end := start + 20
            copy(paramData[:], data[start:end])
            m.parseParameterNormal(pbi, paramData, &tempos)
        }
    }
}

func (m PresetLoad) parseParameterCab(pbi pod.PedalBoardItem, data [20]byte, paramID uint32) {
    param := pbi.GetParam(paramID)
    if param == nil {
        log.Printf("TODO: Parameter ID %d does not exist on item type %s\n",
            paramID, pbi.GetName())
        return
    }
    binValue := [4]byte{}
    copy(binValue[:], data[4:8])
    if err := param.SetBinValueCurrent(binValue); err != nil {
        log.Printf("TODO: Fix the parameter type on pod.%s, parameter %s current : %s \n", pbi.GetName(), param.GetName(), err)
    }
}

func (m PresetLoad) parseParameterNormal(pbi pod.PedalBoardItem, data [20]byte, tempos *[]uint8) {
    paramID := binary.LittleEndian.Uint32(data[0:4])
    param := pbi.GetParam(paramID)
    if param == nil {
        log.Printf("TODO: Parameter ID %d does not exist on pod.type %s\n",
            paramID, pbi.GetName())
        return
    }

    var v float32
    switch param.(type) {
    case *pod.TempoParam:
        var tempo uint8
        tempo, *tempos = (*tempos)[0], (*tempos)[1:]
        if tempo > 1 {
            v = float32(tempo)
            break
        }
        binary.Read(bytes.NewReader(data[4:8]), binary.LittleEndian, &v)
    default:
        binary.Read(bytes.NewReader(data[4:8]), binary.LittleEndian, &v)
    }

    binValue := [4]byte{}
    buf := new(bytes.Buffer)
    binary.Write(buf, binary.LittleEndian, v)
    copy(binValue[:], buf.Bytes())
    if err := param.SetBinValueCurrent(binValue); err != nil {
        log.Printf("TODO: Fix the parameter type on pod.%s, parameter %s current : %s \n", pbi.GetName(), param.GetName(), err)
    }
    copy(binValue[:], data[8:12])
    if err := param.SetBinValueMin(binValue); err != nil {
        log.Printf("TODO: Fix the parameter type on pod.%s, parameter %s min: %s \n", pbi.GetName(), param.GetName(), err)
    }
    copy(binValue[:], data[12:16])
    if err := param.SetBinValueMax(binValue); err != nil {
        log.Printf("TODO: Fix the parameter type on pod.%s, parameter %s max: %s \n", pbi.GetName(), param.GetName(), err)
    }
}

func (m SetChange) Parse(p *pod.Pod) (error, int, interface{}) {
    p.SetCurrentSet(m.data[8])
    return nil, ct.StatusSetChange(), p.GetCurrentSet()
}

func (m SetLoad) Parse(p *pod.Pod) (error, int, interface{}) {
    p.GetCurrentSet().SetName(string(m.data[12:]))
    return nil, ct.StatusSetLoad(), nil
}

func (m SetupChange) Parse(p *pod.Pod) (error, int, interface{}) {
    setupType := binary.LittleEndian.Uint32(m.data[16:20])
    var value [4]byte
    copy(value[:], m.data[20:])
    preset := p.GetCurrentPreset()

    switch setupType {
    case setupMessageCab0ER:
        return m.parseCab(preset, 0, pod.CabERID, value)
    case setupMessageCab1ER:
        return m.parseCab(preset, 1, pod.CabERID, value)
    case setupMessageCab0Mic:
        return m.parseCab(preset, 0, pod.CabMicID, value)
    case setupMessageCab1Mic:
        return m.parseCab(preset, 1, pod.CabMicID, value)
    case setupMessageCab0LoCut:
        return m.parseCab(preset, 0, pod.CabLowCutID, value)
    case setupMessageCab1LoCut:
        return m.parseCab(preset, 1, pod.CabLowCutID, value)
    case setupMessageCab0ResLvl:
        return m.parseCab(preset, 0, pod.CabResLevelID, value)
    case setupMessageCab1ResLvl:
        return m.parseCab(preset, 1, pod.CabResLevelID, value)
    case setupMessageCab0Thump:
        return m.parseCab(preset, 0, pod.CabThumpID, value)
    case setupMessageCab1Thump:
        return m.parseCab(preset, 1, pod.CabThumpID, value)
    case setupMessageCab0Decay:
        return m.parseCab(preset, 0, pod.CabDecayID, value)
    case setupMessageCab1Decay:
        return m.parseCab(preset, 1, pod.CabDecayID, value)
    case setupMessageInput1Source:
        return m.parsePedalBoard(preset, pod.PresetInput1Source, value)
    case setupMessageInput2Source:
        return m.parsePedalBoard(preset, pod.PresetInput2Source, value)
    case setupMessageGuitarInZ:
        return m.parsePedalBoard(preset, pod.PresetGuitarInZ, value)
    case setupMessageTempo:
        return m.parsePedalBoard(preset, pod.PresetTempo, value)
    }

    return nil, ct.StatusNone(), nil
}

func (m SetupChange) parseCab(p *pod.Preset, ID int, paramID uint32, value [4]byte) (error, int, interface{}) {
    c := p.GetCab(ID)
    if c == nil {
        return fmt.Errorf("Can't find Cab %d", ID), ct.StatusWarning(), nil
    }
    param := c.GetParam(paramID)
    if param == nil {
        return fmt.Errorf("Can't get param %d, for Cab %d", paramID, ID), ct.StatusWarning(), nil
    }
    if err := param.SetBinValueCurrent(value); err != nil {
        return fmt.Errorf("Cant set Cab ID %d parameter ID %d value: %s", ID, paramID, err), ct.StatusWarning(), nil
    }
    return nil, ct.StatusParameterChange(), p
}

func (m SetupChange) parsePedalBoard(p *pod.Preset, parameterID uint32, value [4]byte) (error, int, interface{}) {
    param := p.GetParam(parameterID)
    if param == nil {
        return fmt.Errorf("Can't get PedalBoard parameter ID %d", parameterID), ct.StatusWarning(), nil
    }
    if err := param.SetBinValueCurrent(value); err != nil {
        return fmt.Errorf("Cant set PedalBoard parameter ID %d value: %s", parameterID, err), ct.StatusWarning(), nil
    }
    return nil, ct.StatusParameterChange(), p
}

func (m TypeChange) Parse(p *pod.Pod) (error, int, interface{}) {
    id := m.getPedalBoardItemID()
    param := p.GetCurrentPreset().GetItem(id)
    if param == nil {
        return fmt.Errorf("Item ID %d not found", id), ct.StatusWarning(), nil
    }
    ptype := binary.LittleEndian.Uint32(m.data[16:])
    if err := param.SetType(ptype); err != nil {
        return err, ct.StatusWarning(), nil
    }
    return nil, ct.StatusTypeChange(), param
}

func (m StatusResponse) Parse(p *pod.Pod) (error, int, interface{}) {
    status := binary.LittleEndian.Uint32(m.data[12:16])
    value := binary.LittleEndian.Uint32(m.data[16:])

    switch status {
    case statusIDPreset:
        p.SetCurrentPreset(uint8(value))
        return nil, ct.StatusPresetChange(), p.GetCurrentPreset()
    case statusIDSet:
        p.SetCurrentSet(uint8(value))
        return nil, ct.StatusSetChange(), p.GetCurrentSet()
    }

    return nil, ct.StatusNone(), nil
}
