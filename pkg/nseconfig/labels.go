package nseconfig

type Labels map[string]string

func (l Labels) String() string {
	labelString := ""
	for k, v := range l {
		if len(labelString) > 0 {
			labelString += ","
		}
		labelString = labelString + k + "=" + v
	}

	return labelString
}
